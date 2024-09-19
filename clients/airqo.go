package clients

import (
	"airqo-integrator/config"
	"errors"
	"github.com/go-resty/resty/v2"
	log "github.com/sirupsen/logrus"
	"strings"
)

var AirQoClient *Client

var AirQoServer *Server

func init() {
	InitAirQoServer()
	AirQoClient, _ = AirQoServer.NewAirQoClient()
}

func GetAirQoBaseURL(url string) (string, error) {
	if strings.Contains(url, "/api/v2/") {
		pos := strings.Index(url, "/api/v2/")
		return url[:pos], nil
	}
	return url, errors.New("URL doesn't contain /api/v2/ part")
}

func InitAirQoServer() {
	AirQoServer = &Server{
		BaseUrl:    config.AirQoIntegratorConf.API.AIRQOBaseURL,
		Username:   "",
		Password:   "",
		AuthToken:  config.AirQoIntegratorConf.API.AIRQOToken,
		AuthMethod: "Token",
	}
}

func (s *Server) NewAirQoClient() (*Client, error) {
	client := resty.New()
	baseUrl, err := GetAirQoBaseURL(s.BaseUrl)
	if err != nil {
		log.WithFields(log.Fields{
			"URL": s.BaseUrl, "Error": err}).Error("Failed to get base URL from URL")
		return nil, err
	}
	client.SetBaseURL(baseUrl + "/api/v2")
	client.SetHeaders(map[string]string{
		"Accept":       "application/json",
		"Content-Type": "application/json",
		"User-Agent":   "AirQo-DHIS2 Integrator",
	})
	client.SetDisableWarn(true)
	switch s.AuthMethod {
	case "Basic":
		client.SetBasicAuth(s.Username, s.Password)
	case "Token":
		client.SetAuthScheme("Token")
		client.SetAuthToken(s.AuthToken)
	}
	return &Client{
		RestClient: client,
		BaseURL:    baseUrl + "/api/v2",
	}, nil
}
