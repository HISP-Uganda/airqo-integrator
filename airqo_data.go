package main

import (
	"airqo-integrator/config"
	"airqo-integrator/db"
	"airqo-integrator/models"
	"airqo-integrator/utils"
	"encoding/json"
	"fmt"
	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"
)

// MetricsToDataValues takes metrics map[string]float64 and dhis2mappings of map[string]*Dhis2mapping
// and returns a slice of DataValues
func MetricsToDataValues(metrics map[string]float64, dhis2Mappings map[string]*models.Dhis2Mapping) []models.DataValue {
	var dataValues []models.DataValue
	for metricUID, metricValue := range metrics {
		mapping, ok := dhis2Mappings[metricUID]
		if !ok {
			log.WithFields(log.Fields{
				"metricUID": metricUID,
			}).Error("No Dhis2Mapping found for metricUID")
			continue
		}
		dataValue := models.DataValue{
			DataElement:         mapping.DataElement,
			CategoryOptionCombo: mapping.CategoryOptionCombo,
			Value:               utils.FlexString(fmt.Sprintf("%.2f", metricValue)),
		}
		dataValues = append(dataValues, dataValue)
	}
	return dataValues
}

func getSiteDistricts() []int64 {
	if *config.PilotMode {
		d, _ := models.GetOrganisationUnitsByNames(
			strings.Split(config.AirQoIntegratorConf.API.AIRQOPilotDistricts, ","))
		return d
	}
	d, _ := models.GetSiteDistricts()
	return d
}

func getSubCountiesData(subCounties []int64) map[string]any {
	return lo.Reduce(subCounties, func(agg map[string]any, item int64, _ int) map[string]any {
		subCounty, _ := models.GetOrganisationUnitByID(item)
		sites, _ := models.GetSitesByCurrentSubCounty(item)
		siteUIDs := lo.Map(sites, func(item models.Site, _ int) string {
			return item.UID
		})
		data := map[string]any{
			"id":    item,
			"uid":   subCounty.UID,
			"name":  subCounty.Name,
			"sites": siteUIDs,
		}
		agg[subCounty.UID] = data
		return agg
	}, map[string]any{})
}

func updateMetrics(values []float64, min, max, sum *float64, count *int) {
	if len(values) == 0 {
		return
	}

	*min = lo.Min(values)
	*max = lo.Max(values)
	*sum += lo.Sum(values)
	*count += len(values)
}
func allMetricsZero(minPm25, maxPm25, sumPm25 float64, countPm25 int, minPm10, maxPm10, sumPm10 float64, countPm10 int) bool {
	return minPm25 == 0 && maxPm25 == 0 && sumPm25 == 0 && countPm25 == 0 &&
		minPm10 == 0 && maxPm10 == 0 && sumPm10 == 0 && countPm10 == 0
}

func createDataValuesRequest(orgUnitUID, period string, currentTime time.Time,
	dataValues []models.DataValue) models.DataValuesRequest {
	return models.DataValuesRequest{
		OrgUnit:              orgUnitUID,
		Period:               period,
		CompleteDate:         currentTime.Format("2006-01-02"),
		DataSet:              config.AirQoIntegratorConf.API.AIRQODHIS2DataSet,
		AttributeOptionCombo: config.AirQoIntegratorConf.API.AIRQODHIS2AttributeOptionCombo,
		DataValues:           dataValues,
	}
}

func saveRequest(dbConn *sqlx.DB, batchId string, dataValuesRequest models.DataValuesRequest, districtName, subCountyUID string) {
	payload, _ := json.Marshal(dataValuesRequest)
	fmt.Printf("%v\n", string(payload))
	year, week := time.Now().ISOWeek()
	reqF := models.RequestForm{
		Source: "localhost", Destination: "dhis2", ContentType: "application/json",
		Year: fmt.Sprintf("%d", year), Week: fmt.Sprintf("%d", week),
		Month: fmt.Sprintf("%d", int(time.Now().Month())), Period: dataValuesRequest.Period,
		District: districtName, Facility: subCountyUID, BatchID: batchId,
		CCServers: strings.Split(config.AirQoIntegratorConf.API.AIRQOCCDHIS2Servers, ","),
		Body:      string(payload), ObjectType: "AGGREGATE_DATA", ReportType: "airqo_data",
	}

	if _, err := reqF.Save(dbConn); err != nil {
		log.WithError(err).WithFields(log.Fields{"SubCounty": subCountyUID}).Error("Failed to queue - update request for SubCounty")
	}
}

func processSiteMeasurements(sid string, startTime, endTime time.Time,
	minPm25, maxPm25, sumPm25 *float64, countPm25 *int, minPm10, maxPm10, sumPm10 *float64, countPm10 *int) {
	log.Infof("Fetching site measurements: %v. StartDate: %v, EndDate: %v", sid, startTime, endTime)
	mrs, err := models.FetchSiteMeasurements(sid, startTime, endTime)
	if err != nil {
		log.WithError(err).Error("Error fetching site measurements")
		return
	}
	log.Infof("Done fetching measurements for site: %v. StartDate: %v EndDate: %v", sid, startTime, endTime)

	fmt.Printf("Sited ID: %s, Total Measurements: %d\n", sid, len(mrs.Measurements))
	if len(mrs.Measurements) == 0 {
		return
	}

	pm25Values := lo.Map(lo.Filter(mrs.Measurements, func(m models.Measurement, _ int) bool {
		return m.PM25.Value != nil
	}), func(m models.Measurement, _ int) float64 {
		return *m.PM25.Value
	})

	pm10Values := lo.Map(lo.Filter(mrs.Measurements, func(m models.Measurement, _ int) bool {
		return m.PM10.Value != nil
	}), func(m models.Measurement, _ int) float64 {
		return *m.PM10.Value
	})

	updateMetrics(pm25Values, minPm25, maxPm25, sumPm25, countPm25)
	updateMetrics(pm10Values, minPm10, maxPm10, sumPm10, countPm10)
}

func processSubCounty(dbConn *sqlx.DB, batchId string,
	dhis2Mappings map[string]*models.Dhis2Mapping, districtName,
	subCountyUID string, subCountyData map[string]any, startDate, endDate time.Time) {
	subCountyName := subCountyData["name"].(string)
	subCountySites := subCountyData["sites"].([]string)
	// currentDate := startDate

	// Iterate over each day in the specified date range
	//for !currentDate.After(endDate) {
	//	nextDate := currentDate.Add(24 * time.Hour)
	minPm25, maxPm25, sumPm25, countPm25 := 0.0, 0.0, 0.0, 0
	minPm10, maxPm10, sumPm10, countPm10 := 0.0, 0.0, 0.0, 0
	fmt.Printf("Sub County: %s (%s), Sites: %v\n", subCountyName, subCountyUID, subCountySites)

	for _, sid := range subCountySites {
		processSiteMeasurements(sid, startDate, endDate, &minPm25, &maxPm25, &sumPm25, &countPm25, &minPm10,
			&maxPm10, &sumPm10, &countPm10)
	}

	if allMetricsZero(minPm25, maxPm25, sumPm25, countPm25, minPm10, maxPm10, sumPm10, countPm10) {
		log.Infof("No data available for sub county %s (%s) on %v, skipping\n", subCountyName, subCountyUID, startDate.Format("2006-01-02"))
	} else {
		airQoMetrics := map[string]float64{
			"Min PM 2.5":     minPm25,
			"Min PM 10":      minPm10,
			"Max PM 2.5":     maxPm25,
			"Max PM 10":      maxPm10,
			"Average PM 2.5": sumPm25 / float64(countPm25),
			"Average PM 10":  sumPm10 / float64(countPm10),
		}

		dataValues := MetricsToDataValues(airQoMetrics, dhis2Mappings)
		period := startDate.Format("2006-01-02")
		dataValuesRequest := createDataValuesRequest(subCountyData["uid"].(string), period, endDate, dataValues)
		saveRequest(dbConn, batchId, dataValuesRequest, districtName, subCountyUID)
	}

	//	// Move to the next day
	//	currentDate = nextDate
	//}
}

func processDistrict(dbConn *sqlx.DB, batchId string,
	dhis2Mappings map[string]*models.Dhis2Mapping, districtID int64, startDate, endDate time.Time) {
	district, _ := models.GetOrganisationUnitByID(districtID)
	log.Infof("Fetching Measurements of %v", district.Name)
	subCounties, _ := models.GetSubCountiesByDhis2District(districtID)
	subCountiesData := getSubCountiesData(subCounties)

	for subCountyUID, v := range subCountiesData {
		log.Infof("Processing for Subcounty %s: startDate: %v, endDate: %v", subCountyUID, startDate, endDate)
		processSubCounty(dbConn, batchId, dhis2Mappings, district.Name,
			subCountyUID, v.(map[string]any), startDate, endDate)
	}
}

func SendAirQoClimateData2(startDate, endDate time.Time) {
	log.Infof("...::...Starting to fetch and send AirQo data to DHIS2...::...")
	dbConn := db.GetDB()
	batchId := utils.GetUID()
	dhis2Mappings, _ := models.GetDhis2Mappings()
	siteDistricts := getSiteDistricts()
	// Iterate over each day in the specified date range
	for currentDate := startDate; !currentDate.After(endDate); currentDate = currentDate.Add(24 * time.Hour) {
		nextDate := currentDate.Add(24 * time.Hour)
		for _, districtID := range siteDistricts {
			log.Infof("Processing for district %d: startDate %v, endDate: %v", districtID, currentDate, nextDate)
			processDistrict(dbConn, batchId, dhis2Mappings, districtID, currentDate, nextDate)
		}
	}
	log.Infof("...::...Done fetching and sending AirQo data to DHIS2...::...")
}

//func SendAirQoClimateData() {
//	// Fetch climate data from Airqo API
//	dbConn := db.GetDB()
//	batchId := utils.GetUID()
//	dhis2Mappings, _ := models.GetDhis2Mappings()
//	var siteDistricts []int64
//	if *config.PilotMode {
//		siteDistricts, _ = models.GetOrganisationUnitsByNames(
//			strings.Split(config.AirQoIntegratorConf.API.AIRQOPilotDistricts, ","))
//	} else {
//		siteDistricts, _ = models.GetSiteDistricts()
//	}
//	for _, districtID := range siteDistricts {
//		// Get sub counties in each site district
//		district, _ := models.GetOrganisationUnitByID(districtID)
//		log.Infof("Fetching Measurements of %v", district.Name)
//		subCounties, _ := models.GetSubCountiesByDhis2District(districtID)
//		subCountiesData := lo.Reduce(subCounties, func(agg map[string]any, item int64, _ int) map[string]any {
//			subCounty, _ := models.GetOrganisationUnitByID(item)
//			sites, _ := models.GetSitesByCurrentSubCounty(item)
//			siteUIDs := lo.Map(sites, func(item models.Site, _ int) string {
//				return item.UID
//			})
//			data := map[string]any{
//				"id":    item,
//				"uid":   subCounty.UID,
//				"name":  subCounty.Name,
//				"sites": siteUIDs,
//			}
//			agg[subCounty.UID] = data
//			return agg
//		}, map[string]any{})
//		// log.Infof("Sub County Data Details %v", subCountiesData)
//		for subCountyUID, v := range subCountiesData {
//			subCountyName := v.(map[string]any)["name"]
//			fmt.Printf("Sub County: %s (%s), Sites: %v\n", subCountyName, subCountyUID, v.(map[string]any)["sites"])
//			minPm25 := 0.0
//			maxPm25 := 0.0
//			minPm10 := 0.0
//			maxPm10 := 0.0
//			sumPm25 := 0.0
//			sumPm10 := 0.0
//			countPm25 := 0
//			countPm10 := 0
//			subCountySites := v.(map[string]any)["sites"]
//			currentTime := time.Now()
//			yesterday := currentTime.Add(-24 * time.Hour)
//			for _, sid := range subCountySites.([]string) {
//				// fmt.Printf("SiteID: %s\n", sid)
//				mrs, err := models.FetchSiteMeasurements(sid, yesterday, currentTime)
//				if err != nil {
//					log.WithError(err).Error("Error fetching site measurements")
//					continue
//				}
//				fmt.Printf("Sited ID: %s, Total Measurements: %d\n", sid, len(mrs.Measurements))
//				if len(mrs.Measurements) > 0 {
//					pm25max := lo.Max(lo.Map(lo.Filter(mrs.Measurements, func(item models.Measurement, _ int) bool {
//						return item.PM25.Value != nil
//					}), func(item models.Measurement, _ int) float64 {
//						return *item.PM25.Value
//					}))
//					pm10max := lo.Max(lo.Map(lo.Filter(mrs.Measurements, func(item models.Measurement, _ int) bool {
//						return item.PM10.Value != nil
//					}), func(item models.Measurement, _ int) float64 {
//						return *item.PM10.Value
//					}))
//					pm25min := lo.Min(lo.Map(lo.Filter(mrs.Measurements, func(item models.Measurement, _ int) bool {
//						return item.PM25.Value != nil
//					}), func(item models.Measurement, _ int) float64 {
//						return *item.PM25.Value
//					}))
//					pm10min := lo.Min(lo.Map(lo.Filter(mrs.Measurements, func(item models.Measurement, _ int) bool {
//						return item.PM10.Value != nil
//					}), func(item models.Measurement, _ int) float64 {
//						return *item.PM10.Value
//					}))
//
//					// Sum and count for average PM2.5
//					lo.ForEach(lo.Filter(mrs.Measurements, func(item models.Measurement, _ int) bool {
//						return item.PM25.Value != nil
//					}), func(item models.Measurement, _ int) {
//						sumPm25 += *item.PM25.Value
//						countPm25++
//					})
//
//					// Sum and count for average PM10
//					lo.ForEach(lo.Filter(mrs.Measurements, func(item models.Measurement, _ int) bool {
//						return item.PM10.Value != nil
//					}), func(item models.Measurement, _ int) {
//						sumPm10 += *item.PM10.Value
//						countPm10++
//					})
//
//					// Update min and max PM2.5 and PM10 values for the sub county
//					// if pm25min >  current minPm25, update minPm25 with pm25min
//					if pm25min < minPm25 || minPm25 == 0 {
//						minPm25 = pm25min
//					}
//
//					//if pm10min >  current minPm10, update minPm10 with pm10min
//					if pm10min < minPm10 || minPm10 == 0 {
//						minPm10 = pm10min
//					}
//
//					if pm25max > maxPm25 {
//						maxPm25 = pm25max
//					}
//
//					if pm10max > maxPm10 {
//						maxPm10 = pm10max
//					}
//				}
//
//			}
//			// Calculate the average PM2.5 and PM10
//			avgPm25 := 0.0
//			avgPm10 := 0.0
//
//			if countPm25 > 0 {
//				avgPm25 = sumPm25 / float64(countPm25)
//			}
//
//			if countPm10 > 0 {
//				avgPm10 = sumPm10 / float64(countPm10)
//			}
//
//			// if all the metrics are 0 then continue
//			if minPm25 == 0 && minPm10 == 0 && maxPm25 == 0 && maxPm10 == 0 && avgPm25 == 0 && avgPm10 == 0 {
//				log.Infof("No data available for sub county %s (%s), skipping\n", subCountyName, subCountyUID)
//				continue
//			}
//			airQoMetrics := make(map[string]float64)
//			//fmt.Printf("Min PM2.5: %.2f, Max PM2.5: %.2f, Avg PM2.5: %.2f, Min PM10: %.2f, Max PM10: %.2f, Avg PM10: %.2f\n",
//			//	minPm25, maxPm25, avgPm25, minPm10, maxPm10, avgPm10)
//			airQoMetrics["Min PM 2.5"] = minPm25
//			airQoMetrics["Min PM 10"] = minPm10
//			airQoMetrics["Max PM 2.5"] = maxPm25
//			airQoMetrics["Max PM 10"] = maxPm10
//			airQoMetrics["Average PM 2.5"] = avgPm25
//			airQoMetrics["Average PM 10"] = avgPm10
//
//			dataValues := MetricsToDataValues(airQoMetrics, dhis2Mappings)
//			period := yesterday.Format("2006-01-02")
//			dataValuesRequest := models.DataValuesRequest{
//				OrgUnit:              v.(map[string]any)["uid"].(string),
//				Period:               period,
//				CompleteDate:         currentTime.Format("2006-01-02"),
//				DataSet:              config.AirQoIntegratorConf.API.AIRQODHIS2DataSet,
//				AttributeOptionCombo: config.AirQoIntegratorConf.API.AIRQODHIS2AttributeOptionCombo,
//				DataValues:           dataValues,
//			}
//			payload, _ := json.Marshal(dataValuesRequest)
//			fmt.Printf("%v\n", string(payload))
//			year, week := time.Now().ISOWeek()
//			var reqF = models.RequestForm{
//				// DependsOn: dependency,
//				Source: "localhost", Destination: "dhis2", ContentType: "application/json",
//				Year: fmt.Sprintf("%d", year), Week: fmt.Sprintf("%d", week),
//				Month:  fmt.Sprintf("%d", int(time.Now().Month())),
//				Period: period, District: district.Name,
//				Facility: subCountyUID, BatchID: batchId, SubmissionID: "",
//				CCServers: strings.Split(config.AirQoIntegratorConf.API.AIRQOCCDHIS2Servers, ","),
//				URLSuffix: "",
//				Body:      string(payload), ObjectType: "AGGREGATE_DATA", ReportType: "airqo_data",
//			}
//			_, err := reqF.Save(dbConn)
//			if err != nil {
//				log.WithError(err).WithFields(log.Fields{"SubCounty": subCountyUID}).Error(
//					"Failed to queue - update request for SubCounty")
//				continue
//			}
//		}
//		fmt.Println("Sending AirQo climate data to DHIS2...")
//	}
//}
