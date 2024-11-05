package main

import (
	"airqo-integrator/config"
	"airqo-integrator/models"
	"fmt"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

const dueSchedulesSQL = `
SELECT id FROM schedules WHERE next_run_at <= NOW() AND is_active = true AND status = 'ready'
ORDER by id ASC
`

func getDueSchedules(db *sqlx.DB) ([]int64, error) {
	var ids []int64
	err := db.Select(&ids, dueSchedulesSQL)
	return ids, err
}

// ProduceSchedules a function that reads ready schedules by id from the database and sends the Id to a job channel for consumer to receive and process
func ProduceSchedules(
	db *sqlx.DB,
	jobs chan<- int64,
	wg *sync.WaitGroup,
	workingOnMutex *sync.Mutex,
	workingOn map[int64]bool,
) {
	defer wg.Done()
	log.Info("..:::.. Starting to produce due schedules..:::..")
	//dbConn, err := sqlx.Connect("postgres", config.AirQoIntegratorConf.Database.URI)
	//if err != nil {
	//	log.Fatalln("Schedule producer failed to connect to database: %v", err)
	//}
	for {
		rows, err := db.Queryx(dueSchedulesSQL)
		if err != nil {
			log.WithError(err).Error("ERROR READING READY SCHEDULES!!!")
		}

		var schedulesCount = 0
		for rows.Next() {
			schedulesCount += 1
			var scheduleID int64
			err = rows.Scan(&scheduleID)
			if err != nil {
				log.WithError(err).Error("Error reading schedule from queue:")
			}
			jobs <- scheduleID
			workingOnMutex.Lock()
			if _, exists := workingOn[scheduleID]; exists {
				log.WithField("scheduleID", scheduleID).Info("Schedule already in dynamic queue")
				continue
			}
			workingOnMutex.Unlock()
			go func(sched int64) {
				// Let see if we can recover from panics XXX
				defer func() {
					if r := recover(); r != nil {
						fmt.Println("Recovered in Produce", r)

					}
				}()
				workingOnMutex.Lock()
				defer workingOnMutex.Unlock()

				jobs <- sched
				workingOn[sched] = true
				log.Info(fmt.Sprintf("Added Schedule [id: %v]", sched))
			}(scheduleID)
		}
		if err := rows.Err(); err != nil {
			log.WithError(err).Error("Error reading schedules")
		}
		_ = rows.Close()
		if schedulesCount > 0 {
			log.WithField("scheduleAdded", schedulesCount).Info("Fetched Schedules")
		}

		// log.Info(fmt.Sprintf("Schedule producer going to sleep for: %v", config.AirQoIntegratorConf.Server.RequestProcessInterval))
		time.Sleep(
			time.Duration(config.AirQoIntegratorConf.Server.RequestProcessInterval) * time.Second)
	}

}

func ConsumeSchedules(db *sqlx.DB, jobs <-chan int64, wg *sync.WaitGroup, workingOnMutex *sync.RWMutex, workingOn map[int64]bool) {
	defer wg.Done()
	for id := range jobs {
		ProcessSchedule(db, id)
		workingOnMutex.Lock()
		delete(workingOn, id)
		log.WithFields(log.Fields{
			"scheduleID":    id,
			"seenMapLength": len(workingOn),
			// "senMap":        seenMap,
		}).Info("Consumer done with schedule.")
		workingOnMutex.Unlock()
		time.Sleep(1 * time.Second)
	}
}

func ProcessSchedule(db *sqlx.DB, id int64) {
	log.WithField("ScheduleID", id).Info("Processing Schedule")
	schedule, err := models.GetSchedule(db, id)
	if err != nil {
		log.WithError(err).Error("Failed to fetch schedule")
		return
	}
	tx, err := db.Beginx()
	if err != nil {
		log.Fatalln(err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		} else if err != nil {
			_ = tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()

	switch schedule.ScheduleType {
	case "dhis2_async_job_check":
		completed, exists, _ := models.CheckDhis2AsyncJobStatus(schedule)
		if completed {
			taskSummary, err := models.CheckDhis2AsyncJobTaskSummary(tx, schedule)
			if err != nil {
				log.WithError(err).Errorf("Failed to check dhis2 async job: Schedule ID: %v", schedule.ID)
			} else {
				schedule.Status = "completed"
				schedule.Updated = time.Now().In(models.Location)
				err = models.UpdateScheduleTx(tx, schedule)
				if err != nil {
					log.WithError(err).Error("Failed to update schedule")
				}
				// log.Infof("Schedule updated successfully: %v", taskSummary)
				reqObj, er := GetRequestObjectById(db, *schedule.RequestID)
				log.Infof("REQUEST OBJECT: %v, !Nil? %v", reqObj, reqObj != nil)
				if er == nil && reqObj != nil {
					if *schedule.ServerInCC {
						log.Infof("XXXXXX")
						serverStatus := reqObj.CCServersStatus[fmt.Sprintf("%d", reqObj.Destination)].(map[string]interface{})
						summary := fmt.Sprintf(
							"Imported: %d, Updated: %d, Ignored: %d, Deleted: %d, Total: %d",
							taskSummary.ImportCount.Imported,
							taskSummary.ImportCount.Updated,
							taskSummary.ImportCount.Ignored,
							taskSummary.ImportCount.Deleted,
							taskSummary.ImportCount.Total)
						newServerStatus := make(map[string]interface{})
						newServerStatus["errors"] = summary
						newServerStatus["status"] = models.RequestStatusCompleted
						// newServerStatus["statusCode"] = fmt.Sprintf("%d", resp.StatusCode)
						switch serverStatus["retries"].(type) {
						case float64:
							newServerStatus["retries"] = int(serverStatus["retries"].(float64) + 1)
						case int:
							newServerStatus["retries"] = serverStatus["retries"].(int) + 1
						}
						reqObj.CCServersStatus[fmt.Sprintf("%d", reqObj.Destination)] = newServerStatus
						// reqObj.updateCCServerStatus(tx)
						_, _ = tx.NamedExec(`UPDATE requests SET cc_servers_status = :cc_servers_status WHERE id = :id`, reqObj)
					} else {
						reqObj.Retries += 1
						if taskSummary.Status == "SUCCESS" {
							reqObj.Status = models.RequestStatusCompleted
							reqObj.Errors = fmt.Sprintf(
								"Imported: %d, Updated: %d, Ignored: %d, Deleted: %d, Total: %d",
								taskSummary.ImportCount.Imported,
								taskSummary.ImportCount.Updated,
								taskSummary.ImportCount.Ignored,
								taskSummary.ImportCount.Deleted,
								taskSummary.ImportCount.Total)
						} else {
							reqObj.Status = models.RequestStatusFailed
							reqObj.Errors = fmt.Sprintf("%v", taskSummary.ImportConflicts)
						}
						reqObj.updateRequest(tx)
					}
				} else {
					log.Infof("Request object not found while updating async request: %d", *schedule.RequestID)
				}
			}

		} else {
			if exists { // perhaps async request removed from server
				schedule.Status = "ready"
				nextRun := time.Now().Add(
					time.Second * time.Duration(config.AirQoIntegratorConf.Server.Dhis2JobStatusCheckInterval))
				_ = schedule.SetNextRun(tx, nextRun)
			} else {
				schedule.Status = "expired"
			}

			schedule.Updated = time.Now().In(models.Location)
			err = models.UpdateScheduleTx(tx, schedule)
			if err != nil {
				log.WithError(err).Error("Failed to update schedule::")
			}
		}

	case "url":
		log.Info("Handling URL schedule")
	case "sms":
		log.Info("Handling URL schedule")
	case "contact_push":
		log.Info("Handling contact push schedule")
	case "command":
		log.Info("Handling command schedule")
	default:
		log.Info("Unknown schedule")

	}
}

func StartScheduleConsumers(scheduledJobs <-chan int64, wg *sync.WaitGroup, mutex *sync.RWMutex, workingOn map[int64]bool) {
	dbURI := config.AirQoIntegratorConf.Database.URI
	log.Info(fmt.Sprintf("Going to create %d Schedule Consumers. Timezone: %s!!!!!\n",
		config.AirQoIntegratorConf.Server.MaxConcurrent, config.AirQoIntegratorConf.Server.TimeZone))
	numConsumers := 0
	for i := 1; i <= config.AirQoIntegratorConf.Server.MaxConcurrent; i++ {
		newConn, err := sqlx.Connect("postgres", dbURI)
		if err != nil {
			log.WithError(err).Error("Schedule processor failed to connect to database")
		} else {
			log.Info(fmt.Sprintf("Adding Schedule Consumer: %d\n", i))
			wg.Add(1)
			go ConsumeSchedules(newConn, scheduledJobs, wg, mutex, workingOn)
			numConsumers++
		}
	}
	if numConsumers == 0 {
		log.Errorln("Schedule processor failed to connect to database for any of the consumers")
	}
	log.Info(fmt.Sprintf("Created %d schedule jobs consumers", numConsumers))
}
