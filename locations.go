package main

import (
	"airqo-integrator/config"
	"airqo-integrator/db"
	"airqo-integrator/models"
	"airqo-integrator/utils"
	"airqo-integrator/utils/dbutils"
	"encoding/json"
	"fmt"
	"github.com/buger/jsonparser"
	_ "github.com/lib/pq"
	log "github.com/sirupsen/logrus"
	"net/url"
	"strings"
	"time"
)

// LoadOuLevels populates organisation unit levels in our DB from base DHIS2
func LoadOuLevels() {
	var id int
	err := db.GetDB().Get(&id, "SELECT count(*) FROM orgunitlevel")
	if err != nil {
		log.WithError(err).Info("Error reading organisation unit levels:")
		return
	}
	if id != 0 {
		log.WithField("ouCount:", id).Info("Base Organisation units levels found!")
		return
	} else {
		log.WithField("ouCount:", id).Info("Base Organisation units levels not found!")

		apiURL := config.AirQoIntegratorConf.API.AIRQODHIS2BaseURL + "/organisationUnitLevels.json"
		p := url.Values{}
		p.Add("fields", "id,name,level")
		p.Add("paging", "false")

		ouURL := apiURL + "?" + p.Encode()
		fmt.Println(ouURL)
		resp, err := utils.GetWithBasicAuth(ouURL, config.AirQoIntegratorConf.API.AIRQODHIS2User,
			config.AirQoIntegratorConf.API.AIRQODHIS2Password)
		if err != nil {
			log.WithError(err).Info("Failed to fetch organisation unit levels")
			return
		}
		// fmt.Println(string(resp))
		v, _, _, err := jsonparser.Get(resp, "organisationUnitLevels")
		if err != nil {
			log.WithError(err).Error("No organisationUnitLevels found by json parser")
			return
		}
		// fmt.Printf("Entries: %s", v)
		var ouLevels []models.OrgUnitLevel
		err = json.Unmarshal(v, &ouLevels)
		if err != nil {
			fmt.Println("Error unmarshaling orgunit level response body:", err)
			return
		}
		for i := range ouLevels {
			ouLevels[i].UID = ouLevels[i].ID
			log.WithFields(
				log.Fields{"uid": ouLevels[i].ID, "name": ouLevels[i].Name, "level": ouLevels[i].Level}).Info("Creating New Orgunit Level")
			ouLevels[i].NewOrgUnitLevel()
		}
	}
}

// LoadOuGroups populates organisation unit groups in our DB from base DHIS2
func LoadOuGroups() {
	var id int
	err := db.GetDB().Get(&id, "SELECT count(*) FROM orgunitgroup")
	if err != nil {
		log.WithError(err).Info("Error reading organisation unit groups:")
		return
	}
	if id != 0 {
		log.WithField("ouCount:", id).Info("Base Organisation unit groups found!")
		return
	} else {
		log.WithField("ouCount:", id).Info("Base Organisation unit groups not found!")

		apiURL := config.AirQoIntegratorConf.API.AIRQODHIS2BaseURL + "/organisationUnitGroups.json"
		p := url.Values{}
		p.Add("fields", "id,name,shortName")
		p.Add("paging", "false")

		ouURL := apiURL + "?" + p.Encode()
		fmt.Println(ouURL)
		resp, err := utils.GetWithBasicAuth(ouURL, config.AirQoIntegratorConf.API.AIRQODHIS2User,
			config.AirQoIntegratorConf.API.AIRQODHIS2Password)
		if err != nil {
			log.WithError(err).Info("Failed to fetch organisation unit groups")
			return
		}
		v, _, _, err := jsonparser.Get(resp, "organisationUnitGroups")
		if err != nil {
			log.WithError(err).Error("json parser failed to get organisationUnitGroups key")
			return
		}
		var ouGroups []models.OrgUnitGroup
		err = json.Unmarshal(v, &ouGroups)
		if err != nil {
			fmt.Println("Error unmarshaling orgunit groups response body:", err)
			return
		}
		for i := range ouGroups {
			ouGroups[i].UID = ouGroups[i].ID
			if len(ouGroups[i].ShortName) == 0 {
				ouGroups[i].ShortName = ouGroups[i].Name
			}
			log.WithFields(
				log.Fields{"uid": ouGroups[i].ID, "name": ouGroups[i].Name, "level": ouGroups[i].ShortName}).Info("Creating New Orgunit Group")
			ouGroups[i].NewOrgUnitGroup()
		}
	}
}

// LoadAttributes fetches OU related attributes from DHIS2 to our DB
func LoadAttributes() {
	apiURL := config.AirQoIntegratorConf.API.AIRQODHIS2BaseURL + "/attributes.json"
	p := url.Values{}
	fields := `id~rename(uid),name,displayName,code,shortName,valueType`
	p.Add("fields", fields)
	p.Add("paging", "false")
	p.Add("filter", "organisationUnitAttribute:eq:true")
	attrURL := apiURL + "?" + p.Encode()
	resp, err := utils.GetWithBasicAuth(attrURL, config.AirQoIntegratorConf.API.AIRQODHIS2User,
		config.AirQoIntegratorConf.API.AIRQODHIS2Password)
	if err != nil {
		log.WithError(err).Info("Failed to fetch organisation unit attributes")
		return
	}

	v, _, _, err := jsonparser.Get(resp, "attributes")
	if err != nil {
		log.WithError(err).Error("json parser failed to get attributes key")
		return
	}
	var attributes []models.Attribute
	err = json.Unmarshal(v, &attributes)
	if err != nil {
		log.WithError(err).Error("Error unmarshalling attributes to attribute list:")
		return
	}
	for _, attribute := range attributes {
		if attribute.ExistsInDB() {
			log.WithFields(log.Fields{
				"uid": attribute.UID, "name": attribute.Name}).Info("Attribute Exists in DB")
		} else {
			log.WithFields(log.Fields{
				"uid": attribute.UID, "name": attribute.Name}).Info("Creating OU attribute:")
			attribute.OrganisationUnitAttribute = true
			attribute.NewAttribute()
			if len(attribute.Code) > 0 {
				attribute.UpdateCode(attribute.Code)
			}
		}
	}

}

// LoadLocations populates organisation units in our DB from base DHIS2
func LoadLocations() {
	var id int
	err := db.GetDB().Get(&id, "SELECT count(*) FROM organisationunit WHERE hierarchylevel=1")
	if err != nil {
		log.WithError(err).Info("Error reading organisation units:")
		return
	}
	if id != 0 {
		log.WithField("ouCount:", id).Info("Base Organisation units hierarchy found!")
		return
	} else {
		log.WithField("ouCount:", id).Info("Base Organisation units hierarchy not found!")

		apiURL := config.AirQoIntegratorConf.API.AIRQODHIS2BaseURL + "/organisationUnits.json"
		p := url.Values{}
		fields := `id,name,displayName,code,shortName,openingDate,phoneNumber,` +
			`path,level,description,geometry,organisatoinUnitGroups[id,name]`
		p.Add("fields", fields)
		p.Add("paging", "false")

		for i := 1; i < config.AirQoIntegratorConf.API.AIRQODHIS2FacilityLevel; i++ {
			p.Add("level", fmt.Sprintf("%d", i))
			ouURL := apiURL + "?" + p.Encode()
			fmt.Println(ouURL)
			resp, err := utils.GetWithBasicAuth(ouURL, config.AirQoIntegratorConf.API.AIRQODHIS2User,
				config.AirQoIntegratorConf.API.AIRQODHIS2Password)
			if err != nil {
				log.WithError(err).Info("Failed to fetch organisation units")
				return
			}

			v, _, _, err := jsonparser.Get(resp, "organisationUnits")
			if err != nil {
				log.WithError(err).Error("json parser failed to get organisationUnit key")
				return
			}
			var ous []models.OrganisationUnit
			err = json.Unmarshal(v, &ous)
			if err != nil {
				log.WithError(err).Error("Error unmarshalling response body:")
				return
			}
			for i := range ous {
				ous[i].UID = ous[i].ID
				log.WithFields(
					log.Fields{"uid": ous[i].ID, "name": ous[i].Name, "level": ous[i].Level,
						"Geometry-Type": ous[i].Geometry.Type}).Info("Creating New Orgunit")
				ous[i].NewOrgUnit()
			}

			p.Del("level")
		}
	}
}

func SanitizeShortName(name string) string {
	if len(name) <= 50 {
		return name
	}
	newName := strings.ReplaceAll(name, "Health Centre", "HC")
	newName = strings.ReplaceAll(newName, "Hospital", "")
	if len(newName) <= 50 {
		return newName
	}
	return newName[:50]
}

// SyncLocationsToDHIS2Instances syncs the DHIS2 base hierarchy to subscribing DHIS2 instances
func SyncLocationsToDHIS2Instances() {
	for _, serverName := range strings.Split(config.AirQoIntegratorConf.API.AIRQOCCDHIS2HierarchyServers, ",") {
		models.SyncLocationsToServer(serverName)
	}
}

// CreateOrgUnitGroupPayload ...
func CreateOrgUnitGroupPayload(ou models.MetadataOu) map[string][]byte {
	ouGroupReqs := make(map[string][]byte)
	if len(ou.OrganisationUnitGroups) > 0 {
		for _, g := range ou.OrganisationUnitGroups {
			groupId := g.Get("id", "")
			ouGroupReqs[groupId.(string)] = []byte(fmt.Sprintf(
				`[{"op": "add", "path": "/organisationUnits/-", "value": {"id": "%s"}}]`, ou.UID))
		}

	}
	return ouGroupReqs
}

// MakeOrgUnitGroupsAdditionRequests ....
func MakeOrgUnitGroupsAdditionRequests(
	ouGroupPayloads map[string][]byte, dependency dbutils.Int, facilityUID string) []models.RequestForm {
	var requests []models.RequestForm
	for k, v := range ouGroupPayloads {
		if len(k) == 0 {
			continue
		}
		year, week := time.Now().ISOWeek()
		var reqF = models.RequestForm{
			// DependsOn: dependency,
			Source: "localhost", Destination: "base_OU_GroupAdd", ContentType: "application/json-patch+json",
			Year: fmt.Sprintf("%d", year), Week: fmt.Sprintf("%d", week),
			Month: fmt.Sprintf("%d", int(time.Now().Month())), Period: "", Facility: facilityUID, BatchID: "", SubmissionID: "",
			CCServers: strings.Split(config.AirQoIntegratorConf.API.AIRQOCCDHIS2OuGroupAddServers, ","),
			URLSuffix: fmt.Sprintf("/%s", k),
			Body:      string(v), ObjectType: "ORGUNIT_GROUP_ADD", ReportType: "OUGROUP_ADD",
		}
		if dependency > 0 {
			reqF.DependsOn = dependency
		}
		requests = append(requests, reqF)
	}
	return requests
}

func GenerateUpdateMetadataRequest(update []models.MetadataObject, facilityUID string, district string) models.RequestForm {
	req := models.RequestForm{}
	body, err := json.Marshal(update)
	if err != nil {
		log.WithError(err).Error("Failed to parse facility update metadata")
		return req
	}
	year, week := time.Now().ISOWeek()
	reqF := models.RequestForm{
		Source: "localhost", Destination: "base_OU_Update", ContentType: "application/json-patch+json",
		Year: fmt.Sprintf("%d", year), Week: fmt.Sprintf("%d", week),
		Month: fmt.Sprintf("%d", int(time.Now().Month())), Period: "", Facility: facilityUID, BatchID: "", SubmissionID: "",
		District:  district,
		CCServers: strings.Split(config.AirQoIntegratorConf.API.AIRQOCCDHIS2UpdateServers, ","),
		URLSuffix: fmt.Sprintf("/%s", facilityUID),
		Body:      string(body),
	}

	return reqF
}
