package main

import (
	"airqo-integrator/config"
	"airqo-integrator/controllers"
	"airqo-integrator/models"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/robfig/cron/v3"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	log "github.com/sirupsen/logrus"
	"os"
	"sync"
	"time"
)

func init() {
	formatter := new(log.TextFormatter)
	formatter.TimestampFormat = time.RFC3339
	formatter.FullTimestamp = true

	log.SetFormatter(formatter)
	log.SetOutput(os.Stdout)
}

type LocationEntry struct {
	FullURL  string        `json:"fullUrl"`
	Resource fhir.Location `json:"resource"`
}

var splash = `
┏━┓╻┏━┓┏━┓┏━┓   ╺┳╸┏━┓   ╺┳┓╻ ╻╻┏━┓┏━┓
┣━┫┃┣┳┛┃┓┃┃ ┃    ┃ ┃ ┃    ┃┃┣━┫┃┗━┓┏━┛
╹ ╹╹╹┗╸┗┻┛┗━┛    ╹ ┗━┛   ╺┻┛╹ ╹╹┗━┛┗━╸
`

func testing(msg string) {
	log.Infof("Hi %s, You're testing schedules task at %v", msg, time.Now().Format("15:04:05"))
}

func main() {
	fmt.Printf(splash)
	dbConn, err := sqlx.Connect("postgres", config.AirQoIntegratorConf.Database.URI)
	if err != nil {
		log.Fatalln(err)
	}
	// log.WithField("DHIS2_SERVER_CONFIGS", config.MFLDHIS2ServersConfigMap).Info("SERVER: =======>")
	LoadServersFromConfigFiles(config.AIRQODHIS2ServersConfigMap)
	// log.WithFields(log.Fields{"Servers": models.ServerMapByName["localhost"]}).Info("SERVERS==>>")
	// os.Exit(1)

	go func() {
		if !*config.SkipSync {
			LoadOuLevels()
			LoadOuGroups()
			LoadAttributes()
			LoadLocations() // Load organisation units - before facility in base DHIS2 instance
			// SyncLocationsToDHIS2Instances()
		}
		_ = models.LoadSites()
		_ = models.LoadGrids()

		// SendAirQoClimateData()
		if !*config.SkipFectchingByDate {
			startDate, err := time.Parse("2006-01-02", *config.StartDate)
			if err != nil {
				fmt.Println("Error parsing start date:", err)
				return
			}
			endDate, err := time.Parse("2006-01-02", *config.EndDate)
			if err != nil {
				fmt.Println("Error parsing end date:", err)
				return
			}
			SendAirQoClimateData2(startDate, endDate)
		}
	}()

	go func() {
		// Create a new scheduler
		c := cron.New()

		if !*config.SkipSync {
			_, err := c.AddFunc(config.AirQoIntegratorConf.API.AIRQOSyncCronExpression, func() {
				SendAirQoClimateData2(time.Now().Add(-24*time.Hour), time.Now())
			})
			if err != nil {
				log.WithError(err).Error("Error scheduling measurements sync task:")
				return
			}
		}
		if !*config.SkipRequestProcessing {
			_, err := c.AddFunc(config.AirQoIntegratorConf.API.AIRQORetryCronExpression, func() {
				RetryIncompleteRequests()
			})
			if err != nil {
				log.WithError(err).Error("Error scheduling incomplete request retry task:")
			}
		}

		c.Start()
	}()

	jobs := make(chan int)
	var wg sync.WaitGroup

	seenMap := make(map[models.RequestID]bool)
	mutex := &sync.Mutex{}
	rWMutex := &sync.RWMutex{}

	if !*config.SkipRequestProcessing {
		// don't produce anything if skip processing is enabled

		// Start the producer goroutine
		wg.Add(1)
		go Produce(dbConn, jobs, &wg, mutex, seenMap)

		// Start the consumer goroutine
		wg.Add(1)
		go StartConsumers(jobs, &wg, rWMutex, seenMap)
	}
	scheduledJobs := make(chan int64)
	workingOn := make(map[int64]bool)
	var workingOnMutex = &sync.Mutex{}
	var rWworkingOnMutex = &sync.RWMutex{}

	if !*config.SkipScheduleProcessing {
		wg.Add(1)
		go ProduceSchedules(dbConn, scheduledJobs, &wg, workingOnMutex, workingOn)

		wg.Add(1)
		go StartScheduleConsumers(scheduledJobs, &wg, rWworkingOnMutex, workingOn)

	}

	// Start the backend API gin server
	if !*config.DisableHTTPServer {
		wg.Add(1)
		go startAPIServer(&wg)
	}

	wg.Wait()
	close(scheduledJobs)
	close(jobs)
}

func startAPIServer(wg *sync.WaitGroup) {
	defer wg.Done()
	router := gin.Default()
	v2 := router.Group("/api", BasicAuth())
	{
		v2.GET("/test2", func(c *gin.Context) {
			c.String(200, "Authorized")
		})

		tk := new(controllers.TokenController)
		v2.GET("/getToken", tk.GetActiveToken)
		v2.GET("/generateToken", tk.GenerateNewToken)
		v2.DELETE("/deleteTokens", tk.DeleteInactiveTokens)

		q := new(controllers.QueueController)
		v2.POST("/queue", q.Queue)
		v2.GET("/queue", q.Requests)
		v2.GET("/queue/:id", q.GetRequest)
		v2.DELETE("/queue/:id", q.DeleteRequest)

		ou := new(controllers.OrgUnitController)
		v2.POST("/organisationUnits", ou.OrgUnit)
		v2.GET("/organisationUnits", ou.GetOrganisationUnits)

		s := new(controllers.ServerController)
		v2.POST("/servers", s.CreateServer)
		v2.POST("/importServers", s.ImportServers)

		ot := new(controllers.OrgUnitTreeController)
		v2.GET("/outree/:server", ot.CreateOrgUnitTree)

		at := new(controllers.AttributeController)
		v2.GET("/syncAttributes/:server", at.SyncAttributes)

		ad := new(controllers.AdminController)
		v2.GET("/clearDistrictRequests/:district", ad.ClearRequestsByDistrict)
		v2.GET("/clearBatchRequests/:batch", ad.ClearRequestsByBatch)

		sc := new(controllers.ScheduleController)
		v2.GET("/schedules", sc.ListSchedules)
		v2.POST("/schedules", sc.NewSchedule)
		v2.GET("/schedules/:id", sc.GetSchedule)
		v2.POST("/schedules/:id", sc.UpdateSchedule)
		v2.DELETE("/schedules/:id", sc.DeleteSchedule)

	}
	// Handle error response when a route is not defined
	router.NoRoute(func(c *gin.Context) {
		c.String(404, "Page Not Found!")
	})

	_ = router.Run(":" + fmt.Sprintf("%s", config.AirQoIntegratorConf.Server.Port))
}

//TIP See GoLand help at <a href="https://www.jetbrains.com/help/go/">jetbrains.com/help/go/</a>.
// Also, you can try interactive lessons for GoLand by selecting 'Help | Learn IDE Features' from the main menu.
