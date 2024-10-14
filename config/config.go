package config

import (
	goflag "flag"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/lib/pq"
	"github.com/spf13/viper"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
)

// AirQoIntegratorConf is the global conf
var AirQoIntegratorConf Config
var ForceSync *bool
var SkipOUSync *bool
var PilotMode *bool
var StartDate *string
var EndDate *string
var DisableHTTPServer *bool
var SkipRequestProcessing *bool // used to ignore the attempt to send request. Don't produce or consume requests
var SkipScheduleProcessing *bool
var SkipFectchingByDate *bool
var AIRQODHIS2ServersConfigMap = make(map[string]ServerConf)
var ShowVersion *bool

const VERSION = "1.0.0"

// var FakeSyncToBaseDHIS2 *bool

func init() {
	// ./airqo-integrator --config-file /etc/airqointegrator/airqod.yml
	var configFilePath, configDir, conf_dDir string
	currentOS := runtime.GOOS
	switch currentOS {
	case "windows":
		configDir = "C:\\ProgramData\\AirQoIntegrator"
		configFilePath = "C:\\ProgramData\\AirQoIntegrator\\airqod.yml"
		conf_dDir = "C:\\ProgramData\\AirQoIntegrator\\conf.d"
	case "darwin", "linux":
		configFilePath = "/etc/airqo-integrator/airqod.yml"
		configDir = "/etc/airqo-integrator/"
		conf_dDir = "/etc/airqo-integrator/conf.d" // for the conf.d directory where to dump server confs
	default:
		fmt.Println("Unsupported operating system")
		return
	}

	configFile := flag.String("config-file", configFilePath,
		"The path to the configuration file of the application")

	startDate := time.Now().Add(-24 * time.Hour).Format("2006-01-02")
	endDate := time.Now().Format("2006-01-02")
	ForceSync = flag.Bool("force-sync", false, "Whether to forcefully sync organisation unit hierarchy")
	SkipOUSync = flag.Bool("skip-ousync", false, "Whether to skip ou and facility sync. But process requests")
	PilotMode = flag.Bool("pilot-mode", false, "Whether we're running integrator in pilot mode")
	StartDate = flag.String("start-date", startDate, "Date from which to start fetching data (YYYY-MM-DD)")
	EndDate = flag.String("end-date", endDate, "Date until which to fetch data (YYYY-MM-DD)")
	DisableHTTPServer = flag.Bool("disable-http-server", false, "Whether to disable HTTP Server")
	SkipRequestProcessing = flag.Bool("skip-request-processing", false, "Whether to skip requests processing")
	SkipScheduleProcessing = flag.Bool("skip-schedule-processing", false, "Whether to skip schedule processing")
	SkipFectchingByDate = flag.Bool("skip-fetching-by-date", false, "Whether to skip fetching measurements by start and end date")
	ShowVersion = flag.Bool("version", false, "Display version of AIRQO Integrator")
	// FakeSyncToBaseDHIS2 = flag.Bool("fake-sync-to-base-dhis2", false, "Whether to fake sync to base DHIS2")

	flag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	flag.Parse()
	if *ShowVersion {
		fmt.Println("AIRQO Integrator: ", VERSION)
		os.Exit(1)
	}

	viper.SetConfigName("airqod")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(configDir)

	if len(*configFile) > 0 {
		viper.SetConfigFile(*configFile)
		// log.Printf("Config File %v", *configFile)
	}

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// log.Fatalf("Configuration File: %v Not Found", *configFile, err)
			panic(fmt.Errorf("Fatal Error %w \n", err))

		} else {
			log.Fatalf("Error Reading Config: %v", err)

		}
	}

	err := viper.Unmarshal(&AirQoIntegratorConf)
	if err != nil {
		log.Fatalf("unable to decode into struct, %v", err)
	}

	viper.OnConfigChange(func(e fsnotify.Event) {
		fmt.Println("Config file changed:", e.Name)
		err = viper.ReadInConfig()
		if err != nil {
			log.Fatalf("unable to reread configuration into global conf: %v", err)
		}
		_ = viper.Unmarshal(&AirQoIntegratorConf)
	})
	viper.WatchConfig()

	v := viper.New()
	v.SetConfigType("json")

	fileList, err := getFilesInDirectory(conf_dDir)
	if err != nil {
		log.WithError(err).Info("Error reading directory")
	}
	// Loop through the files and read each one
	for _, file := range fileList {
		v.SetConfigFile(file)

		if err := v.ReadInConfig(); err != nil {
			log.WithError(err).WithField("File", file).Error("Error reading config file:")
			continue
		}

		// Unmarshal the config data into your structure
		var config ServerConf
		if err := v.Unmarshal(&config); err != nil {
			log.WithError(err).WithField("File", file).Error("Error unmarshaling config file:")
			continue
		}
		AIRQODHIS2ServersConfigMap[config.Name] = config

		// Now you can use the config structure as needed
		// fmt.Printf("Configuration from %s: %+v\n", file, config)
	}
	v.OnConfigChange(func(e fsnotify.Event) {
		if err := v.ReadInConfig(); err != nil {
			log.WithError(err).WithField("File", e.Name).Error("Error reading config file:")
		}

		// Unmarshal the config data into your structure
		var config ServerConf
		if err := v.Unmarshal(&config); err != nil {
			log.WithError(err).WithField("File", e.Name).Fatalf("Error unmarshaling config file:")
		}
		AIRQODHIS2ServersConfigMap[config.Name] = config
	})
	v.WatchConfig()
}

// Config is the top level cofiguration object
type Config struct {
	Database struct {
		URI string `mapstructure:"uri" env:"AIRQOINTEGRATOR_DB" env-default:"postgres://postgres:postgres@localhost/airqodb?sslmode=disable"`
	} `yaml:"database"`

	Server struct {
		Host                        string `mapstructure:"host" env:"AIRQOINTEGRATOR_HOST" env-default:"localhost"`
		Port                        string `mapstructure:"http_port" env:"AIRQOINTEGRATOR_SERVER_PORT" env-description:"Server port" env-default:"9090"`
		ProxyPort                   string `mapstructure:"proxy_port" env:"AIRQOINTEGRATOR_PROXY_PORT" env-description:"Server port" env-default:"9191"`
		MaxRetries                  int    `mapstructure:"max_retries" env:"AIRQOINTEGRATOR_MAX_RETRIES" env-default:"3"`
		StartOfSubmissionPeriod     string `mapstructure:"start_submission_period" env:"AIRQOINTEGRATOR_START_SUBMISSION_PERIOD" env-default:"18"`
		EndOfSubmissionPeriod       string `mapstructure:"end_submission_period" env:"AIRQOINTEGRATOR_END_SUBMISSION_PERIOD" env-default:"24"`
		MaxConcurrent               int    `mapstructure:"max_concurrent" env:"AIRQOINTEGRATOR_MAX_CONCURRENT" env-default:"5"`
		SkipRequestProcessing       bool   `mapstructure:"skip_request_processing" env:"AIRQOINTEGRATOR_SKIP_REQUEST_PROCESSING" env-default:"false"`
		ForceSync                   bool   `mapstructure:"force_sync" env:"AIRQOINTEGRATOR_FORCE_SYNC" env-default:"false"` // Assume OU hierarchy already there
		SyncOn                      bool   `mapstructure:"sync_on" env:"AIRQOINTEGRATOR_SYNC_ON" env-default:"true"`
		FakeSyncToBaseDHIS2         bool   `mapstructure:"fake_sync_to_base_dhis2" env:"AIRQOINTEGRATOR_FAKE_SYNC_TO_BASE_DHIS2" env-default:"false"`
		RequestProcessInterval      int    `mapstructure:"request_process_interval" env:"AIRQOINTEGRATOR_REQUEST_PROCESS_INTERVAL" env-default:"4"`
		Dhis2JobStatusCheckInterval int    `mapstructure:"dhis2_job_status_check_interval" env:"DHIS2_JOB_STATUS_CHECK_INTERVAL" env-description:"The DHIS2 job status check interval in seconds" env-default:"30"`
		LogDirectory                string `mapstructure:"logdir" env:"AIRQOINTEGRATOR_LOGDIR" env-default:"/var/log/airqointegrator"`
		MigrationsDirectory         string `mapstructure:"migrations_dir" env:"AIRQOINTEGRATOR_MIGRATTIONS_DIR" env-default:"file:///usr/share/airqointegrator/db/migrations"`
		UseSSL                      string `mapstructure:"use_ssl" env:"AIRQOINTEGRATOR_USE_SSL" env-default:"true"`
		SSLClientCertKeyFile        string `mapstructure:"ssl_client_certkey_file" env:"SSL_CLIENT_CERTKEY_FILE" env-default:""`
		SSLServerCertKeyFile        string `mapstructure:"ssl_server_certkey_file" env:"SSL_SERVER_CERTKEY_FILE" env-default:""`
		SSLTrustedCAFile            string `mapstructure:"ssl_trusted_cafile" env:"SSL_TRUSTED_CA_FILE" env-default:""`
		TimeZone                    string `mapstructure:"timezone" env:"DISPATCHER2_TIMEZONE" env-default:"Africa/Kampala" env-description:"The time zone used for this dispatcher2 deployment"`
	} `yaml:"server"`

	API struct {
		AIRQOBaseURL                   string `mapstructure:"airqo_base_url" env:"AIRQOINTEGRATOR_BASE_URL" env-description:"The AIRQO base API URL"`
		AIRQOToken                     string `mapstructure:"airqo_token"  env:"AIRQOINTEGRATOR_TOKEN" env-description:"The AIRQO API token"`
		AIRQOPilotDistricts            string `mapstructure:"airqo_pilot_districts" env:"AIRQOINTEGRATOR_PILOT_DISTRICTS" env-description:"The AIRQO Integration pilot districts" env-default:"Kampala District"`
		AIRQODHIS2Country              string `mapstructure:"airqo_dhis2_country" env:"AIRQOINTEGRATOR_DHIS2_COUNTRY" env-description:"The AIRQO base DHIS2 Country"`
		AIRQODHIS2BaseURL              string `mapstructure:"airqo_dhis2_base_url" env:"AIRQOINTEGRATOR_DHIS2_BASE_URL" env-description:"The AIRQO base DHIS2 instance base API URL"`
		AIRQODHIS2User                 string `mapstructure:"airqo_dhis2_user"  env:"AIRQOINTEGRATOR_DHIS2_USER" env-description:"The AIRQO base DHIS2 username"`
		AIRQODHIS2Password             string `mapstructure:"airqo_dhis2_password"  env:"AIRQOINTEGRATOR_DHIS2_PASSWORD" env-description:"The AIRQO base DHIS2  user password"`
		AIRQODHIS2PAT                  string `mapstructure:"airqo_dhis2_pat"  env:"AIRQOINTEGRATOR_DHIS2_PAT" env-description:"The AIRQO base DHIS2  Personal Access Token"`
		AIRQODHIS2DataSet              string `mapstructure:"airqo_dhis2_dataset"  env:"AIRQOINTEGRATOR_DHIS2_DATASET" env-description:"The AIRQO base DHIS2 DATASET"`
		AIRQODHIS2AttributeOptionCombo string `mapstructure:"airqo_dhis2_attribute_option_combo"  env:"AIRQOINTEGRATOR_DHIS2_ATTRIBUTE_OPTION_COMBO" env-description:"The AIRQO base DHIS2 Attribute Option Combo"`
		AIRQODHIS2AuthMethod           string `mapstructure:"airqo_dhis2_auth_method"  env:"AIRQOINTEGRATOR_DHIS2_AUTH_METHOD" env-description:"The AIRQO base DHIS2  Authentication Method"`
		AIRQODHIS2TreeIDs              string `mapstructure:"airqo_dhis2_tree_ids"  env:"AIRQOINTEGRATOR_DHIS2_TREE_IDS" env-description:"The AIRQO base DHIS2  orgunits top level ids"`
		AIRQODHIS2FacilityLevel        int    `mapstructure:"airqo_dhis2_facility_level"  env:"AIRQOINTEGRATOR_DHIS2_FACILITY_LEVEL" env-description:"The base DHIS2  Orgunit Level for health facilities" env-default:"5"`
		AIRQODHIS2DistrictLevelName    string `mapstructure:"airqo_dhis2_district_oulevel_name"  env:"AIRQOINTEGRATOR_DHIS2_DISTRICT_OULEVEL_NAME" env-description:"The AIRQO base DHIS2 OU Level name for districts" env-default:"District/City"`
		AIRQODHIS2OUAIRQOIDAttributeID string `mapstructure:"airqo_dhis2_ou_airqoid_attribute_id" env:"AIRQOINTEGRATOR_DHIS2_OU_AIRQOID_ATTRIBUTE_ID" env-description:"The DHIS2 OU AIRQOID Attribute ID"`
		AIRQOCCDHIS2Servers            string `mapstructure:"airqo_cc_dhis2_servers"  env:"AIRQOINTEGRATOR_CC_DHIS2_SERVERS" env-description:"The CC DHIS2 instances to receive copy of facilities"`
		AIRQOCCDHIS2HierarchyServers   string `mapstructure:"airqo_cc_dhis2_hierarchy_servers"  env:"AIRQOINTEGRATOR_CC_DHIS2_HIERARCHY_SERVERS" env-description:"The AIRQO CC DHIS2 instances to receive copy of OU hierarchy"`
		AIRQOCCDHIS2CreateServers      string `mapstructure:"airqo_cc_dhis2_create_servers"  env:"AIRQOINTEGRATOR_CC_DHIS2_CREATE_SERVERS" env-description:"The AIRQO CC DHIS2 instances to receive copy of OU creations"`
		AIRQOCCDHIS2UpdateServers      string `mapstructure:"airqo_cc_dhis2_update_servers"  env:"AIRQOINTEGRATOR_CC_DHIS2_UPDATE_SERVERS" env-description:"The AIRQO CC DHIS2 instances to receive copy of OU updates"`
		AIRQOCCDHIS2OuGroupAddServers  string `mapstructure:"airqo_cc_dhis2_ougroup_add_servers"  env:"AIRQOINTEGRATOR_CC_DHIS2_OUGROUP_ADD_SERVERS" env-description:"The AIRQO CC DHIS2 instances APIs used to add ous to groups"`
		AIRQOMetadataBatchSize         int    `mapstructure:"airqo_metadata_batch_size"  env:"AIRQOINTEGRATOR_METADATA_BATCH_SIZE" env-description:"The AIRQO Metadata items to chunk in a metadata request" env-default:"50"`
		AIRQOSyncCronExpression        string `mapstructure:"airqo_sync_cron_expression"  env:"AIRQOINTEGRATOR_SYNC_CRON_EXPRESSION" env-description:"The AIRQO Health Facility Syncronisation Cron Expression" env-default:"0 0-23/6 * * *"`
		AIRQORetryCronExpression       string `mapstructure:"airqo_retry_cron_expression"  env:"AIRQOINTEGRATOR_RETRY_CRON_EXPRESSION" env-description:"The AIRQO request retry Cron Expression" env-default:"*/5 * * * *"`
		AuthToken                      string `mapstructure:"authtoken" env:"RAPIDPRO_AUTH_TOKEN" env-description:"API JWT authorization token"`
	} `yaml:"api"`
}

type ServerConf struct {
	ID                      int64          `mapstructure:"id" json:"-"`
	UID                     string         `mapstructure:"uid" json:"uid,omitempty"`
	Name                    string         `mapstructure:"name" json:"name" validate:"required"`
	Username                string         `mapstructure:"username" json:"username"`
	Password                string         `mapstructure:"password" json:"password,omitempty"`
	IsProxyServer           bool           `mapstructure:"isProxyserver" json:"isProxyServer,omitempty"`
	SystemType              string         `mapstructure:"systemType" json:"systemType,omitempty"`
	EndPointType            string         `mapstructure:"endpointType" json:"endPointType,omitempty"`
	AuthToken               string         `mapstructure:"authToken" db:"auth_token" json:"AuthToken"`
	IPAddress               string         `mapstructure:"IPAddress"  json:"IPAddress"`
	URL                     string         `mapstructure:"URL" json:"URL" validate:"required,url"`
	CCURLS                  pq.StringArray `mapstructure:"CCURLS" json:"CCURLS,omitempty"`
	CallbackURL             string         `mapstructure:"callbackURL" json:"callbackURL,omitempty"`
	HTTPMethod              string         `mapstructure:"HTTPMethod" json:"HTTPMethod" validate:"required"`
	AuthMethod              string         `mapstructure:"AuthMethod" json:"AuthMethod" validate:"required"`
	AllowCallbacks          bool           `mapstructure:"allowCallbacks" json:"allowCallbacks,omitempty"`
	AllowCopies             bool           `mapstructure:"allowCopies" json:"allowCopies,omitempty"`
	UseAsync                bool           `mapstructure:"useAsync" json:"useAsync,omitempty"`
	UseSSL                  bool           `mapstructure:"useSSL" json:"useSSL,omitempty"`
	ParseResponses          bool           `mapstructure:"parseResponses" json:"parseResponses,omitempty"`
	SSLClientCertKeyFile    string         `mapstructure:"sslClientCertkeyFile" json:"sslClientCertkeyFile"`
	StartOfSubmissionPeriod int            `mapstructure:"startSubmissionPeriod" json:"startSubmissionPeriod"`
	EndOfSubmissionPeriod   int            `mapstructure:"endSubmissionPeriod" json:"endSubmissionPeriod"`
	XMLResponseXPATH        string         `mapstructure:"XMLResponseXPATH"  json:"XMLResponseXPATH"`
	JSONResponseXPATH       string         `mapstructure:"JSONResponseXPATH" json:"JSONResponseXPATH"`
	Suspended               bool           `mapstructure:"suspended" json:"suspended,omitempty"`
	URLParams               map[string]any `mapstructure:"URLParams" json:"URLParams,omitempty"`
	Created                 time.Time      `mapstructure:"created" json:"created,omitempty"`
	Updated                 time.Time      `mapstructure:"updated" json:"updated,omitempty"`
	AllowedSources          []string       `mapstructure:"allowedSources" json:"allowedSources,omitempty"`
}

func getFilesInDirectory(directory string) ([]string, error) {
	var files []string

	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".json") {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}
