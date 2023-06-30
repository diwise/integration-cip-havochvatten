package models

import (
	"strconv"
	"time"
)

type Detail struct {
	NutsCode    string  `json:"nutsCode"`
	Name        string  `json:"locationName"`
	Area        string  `json:"locationArea"`
	Description string  `json:"bathInformation"`
	SampleDate  *int64  `json:"sampleDate"`
	Temperature *string `json:"sampleTemperature"`
}

func (d Detail) Date() time.Time {
	if d.Temperature == nil {
		return time.Time{}
	}
	return time.Unix(*d.SampleDate/1000, 0)
}

type TestResult struct {
	//Provtagningsdatum
	SampleDate time.Time `json:"sampleDate"`
	//Vattentemperatur, enhet: °C
	TempValue string `json:"tempValue"`
}

type DetailWithTestResults struct {
	Detail
	TestResults []TestResult `json:"testResult"`
}

type BathWaterProfile struct {
	Name                string      `json:"name"`
	Description         string      `json:"description"`
	NutsCode            string      `json:"nutsCode"`
	ProfileLatestUpdate int64       `json:"profileLatestUpdate"`
	CoperSmhi           []CoperSmhi `json:"coperSmhi"`
	Lat                 float64     `json:"decLat"`
	Long                float64     `json:"decLong"`
}

type CoperSmhi struct {
	//Vattentemperatur för den aktuella tidpunkten, angiven i measHour
	CopernicusData string `json:"copernicusData"`
	//Lufttemperatur för den aktuella tidpunkten, angiven i measHour
	SmhiTemp string `json:"smhiTemp"`
	//Vindstyrka enligt SMHI vid aktuell tidpunkt, angiven i measHour
	SmhiWs string `json:"smhiWs"`
	//Medelnederbörd per timme [mm/h] vid aktuell tidpunkt, angiven i measHour
	SmhiWsymb string `json:"smhiWsymb"`
	//Vädersituation angivet som ett tal mellan 1 - 27 vid aktuell tidpunkt, angiven i measHour
	SmhiPmean string `json:"smhiPmean"`
	//Vindriktning (0 – 359°) enligt SMHI vid aktuell tidpunkt, angiven i measHour
	SmhiWindDir string `json:"smhiWindDir"`
	//Klockslag som prognosen avser, endast timmen på 24h format
	MeasHour string `json:"measHour"`
}

func (c CoperSmhi) Date() (time.Time, bool) {
	if hour, err := strconv.Atoi(c.MeasHour); err == nil {
		year := time.Now().Year()
		month := time.Now().Month()
		day := time.Now().Day()
		loc := time.Local
		m := time.Date(year, month, day, hour, 0, 0, 0, loc)

		return m, true
	}
	return time.Time{}, false
}

type Temperature struct {
	NutsCode   string
	InternalID string
	Lat        float64
	Lon        float64
	Date       time.Time
	Temp       float64
	Source     string
}
