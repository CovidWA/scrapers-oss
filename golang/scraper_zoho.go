package csg

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
)

const ScraperTypeZoho = "zoho"

var ZohoUrlPattern = regexp.MustCompile(`(?i)https?://[^\.\s"]+\.zohobookings\.com/(?:[#a-z0-9]+/)*[a-z0-9]+/?`)
var ZohoDomainArgsIdPattern = regexp.MustCompile(`(?i)https?://([^\.\s"]+\.zohobookings\.com)/(?:[#a-z0-9]+/)*([a-z0-9]+)/?`)
var ZohoOwnerPattern = regexp.MustCompile(`(?i)"appowner":"([^"]+)"`)
var ZohoCsrfNamePattern = regexp.MustCompile(`(?i)"CSRF_PARAM":"([a-z0-9\-]+)"`)
var ZohoCsrfValuePattern = regexp.MustCompile(`(?i)"CSRF_TOKEN":"([a-z0-9\-]+)"`)

const ZohoExecuteUrl = "https://%s/service/api/v1/%s/bookings/functions/BusinessSetupTab/identifyUrlById/execute"
const ZohoExecuteBody = "args-id=%s&%s=%s&functionname=identifyUrlById&namespace=BusinessSetupTab&appLinkName=bookings"

const ZohoServicingUrl = "https://%s/service/api/v2/%s/bookings/view/WEB_SERVICING_STAFF/viewrecords?zc_ownername=%s&SERVICE_ID=[%s]&SERVICE_ID_op=26&%s=%s&deviceType=1&setCriteria=false&removeChanges=true&AGENT_TYPE=ZohoBookings&fromIDX=1&toIDX=950"

const ZohoPrefsUrl = "https://%s/service/api/v2/%s/bookings/view/WEB_CUSTOMER_BOOKING_SETTING/viewrecords?zc_ownername=%s&SETTING_ID=[%s]&SETTING_ID_op=26&SETTING_KEY=[BOOKING_PREFERENCE,CALENDAR_PREFERENCE,SCHEDULING_POLICY]&SETTING_KEY_op=26&%s=%s&deviceType=1&setCriteria=false&removeChanges=true&AGENT_TYPE=ZohoBookings&fromIDX=1&toIDX=950"

const ZohoScheduleUrl = `https://%s/service/api/v2/%s/bookings/view/WEB_BUSINESS_ALL_SCHEDULE/viewrecords?zc_ownername=%s&SCHEDULE_ID=[%s]&SCHEDULE_ID_op=18&FROM=[%%22%s%%22]&FROM_op=20&TO=[%%22%s%%22]&TO_op=21&isForBooking=[true]&isForBooking_op=26&%s=%s&setCriteria=false&removeChanges=true&AGENT_TYPE=ZohoBookings&fromIDX=1&toIDX=950`
const ZohoTimeFormat = "02-Jan-2006 15:04:05"
const ZohoTimeFormatSlot = "02-Jan-2006 03:04 pm"

const ZohoBlackoutUrl = `https://%s/service/api/v2/%s/bookings/view/WEB_INTEG_APPOINTMENT/viewrecords?zc_ownername=%s&REFERENCE_ID=[%s]&REFERENCE_ID_op=26&FROM_DATE_TIME=[%%22%s%%22]&FROM_DATE_TIME_op=20&TO_DATE_TIME=[%%22%s%%22]&TO_DATE_TIME_op=21&%s=%s&deviceType=1&setCriteria=false&removeChanges=true&AGENT_TYPE=ZohoBookings&fromIDX=1&toIDX=950`

var ZohoDefaultTimeZone = "America/Los_Angeles"

type ScraperZoho struct {
	ScraperName  string
	Url          string
	AlternateUrl string
	TimeZone     *time.Location
}

type ScraperZohoFactory struct {
}

func (sf *ScraperZohoFactory) Type() string {
	return ScraperTypeZoho
}

func (sf *ScraperZohoFactory) CreateScrapers(name string) (map[string]Scraper, error) {
	if name == "zoho" {
		//scrapers from airtable

		clinics, err := GetClinicsByKeyPattern(regexp.MustCompile(`^zoho_.+$`))
		if err != nil {
			return nil, err
		}
		scrapers := make(map[string]Scraper)

		for _, clinic := range clinics {
			scraper := new(ScraperZoho)
			scraper.ScraperName = clinic.ApiKey
			scraper.Url = clinic.Url
			scraper.AlternateUrl = clinic.AlternateUrl
			scraper.TimeZone, _ = time.LoadLocation(ZohoDefaultTimeZone)

			scrapers[scraper.Name()] = scraper
		}

		return scrapers, nil
	} else {
		//scrapers from yaml
		scraper := new(ScraperZoho)
		scraper.ScraperName = name
		scraper.TimeZone, _ = time.LoadLocation(ZohoDefaultTimeZone)

		return map[string]Scraper{name: scraper}, nil
	}
}

func (s *ScraperZoho) Type() string {
	return ScraperTypeZoho
}

func (s *ScraperZoho) Name() string {
	return s.ScraperName
}

func (s *ScraperZoho) Configure(params map[string]interface{}) error {
	//TODO: Make timezone configurable

	url, exists := getStringOptional(params, "url")
	if exists && len(url) > 0 {
		s.Url = url
	}
	return nil
}

func (s *ScraperZoho) Scrape() (status Status, tags TagSet, body []byte, err error) {
	return s.ScrapeUrls(s.AlternateUrl, s.Url)
}

func (s *ScraperZoho) ScrapeUrls(urls ...string) (status Status, tags TagSet, body []byte, err error) {
	status = StatusUnknown

	ctx, body, err := s.GetArguments(urls...)
	if err != nil {
		return
	}

	body, err = s.GetIds(ctx)
	if err != nil {
		return
	}

	if len(ctx.WorkspaceIds) == 0 {
		Log.Debugf("%s: No active workspaces found", s.Name())
		status = StatusNo
		return
	} else {
		Log.Debugf("%s: Fetching availability for %d workspace(s) found", s.Name(), len(ctx.WorkspaceIds))
	}

	body, err = s.GetApptPrefs(ctx)
	if err != nil {
		return
	}

	available := false
	available, body, err = s.GetAvailability(ctx)
	if err != nil {
		return
	}

	if available {
		status = StatusYes
	} else {
		status = StatusNo
	}

	return
}

type ZohoScrapeContext struct {
	Domain             string
	ArgsId             string
	Owner              string
	CsrfName           string
	CsrfValue          string
	BusinessId         string
	ServiceIds         []string
	WorkspaceIds       map[string]string //WorkspaceId -> ServiceId
	ScheduleIds        map[string]string //ScheduleId -> WorkspaceId
	BusinessApptPrefs  *ZohoApptPrefs
	WorkspaceApptPrefs map[string]*ZohoApptPrefs //WorkspaceId -> prefs
	Headers            []Header
}

func (ctx *ZohoScrapeContext) GetApptPrefs(workspaceId string) *ZohoApptPrefs {
	apptPrefs := new(ZohoApptPrefs)

	apptPrefs.BookingStart = ctx.BusinessApptPrefs.BookingStart
	apptPrefs.BookingEnd = ctx.BusinessApptPrefs.BookingEnd
	apptPrefs.Duration = ctx.BusinessApptPrefs.Duration

	workspacePref, exists := ctx.WorkspaceApptPrefs[workspaceId]
	if exists {
		if workspacePref.BookingStart >= 0 {
			apptPrefs.BookingStart = workspacePref.BookingStart
		}
		if workspacePref.BookingEnd >= 0 {
			apptPrefs.BookingEnd = workspacePref.BookingEnd
		}
		if workspacePref.Duration >= 0 {
			apptPrefs.Duration = workspacePref.Duration
		}
	}

	if apptPrefs.BookingStart < 0 || apptPrefs.BookingEnd < 0 || apptPrefs.Duration < 0 {
		//invalid prefs
		return nil
	} else {
		return apptPrefs
	}
}

type ZohoApptPrefs struct {
	BookingStart int //start time of available bookings in minutes (0 = now)
	BookingEnd   int //end time of available bookings in minutes
	Duration     int //duration of appointment in minutes
}

func NewZohoApptPrefs() *ZohoApptPrefs {
	apptPrefs := new(ZohoApptPrefs)
	apptPrefs.BookingStart = -1
	apptPrefs.BookingEnd = -1
	apptPrefs.Duration = -1

	return apptPrefs
}

func (s *ScraperZoho) GetArguments(urls ...string) (ctx *ZohoScrapeContext, body []byte, err error) {
	url, body, err := ExtractScrapeUrl(s.Name(), ZohoUrlPattern, urls...)
	if err != nil {
		return
	}

	if body == nil {
		endpoint := new(Endpoint)
		endpoint.Url = url
		endpoint.Method = "GET"
		body, _, err = endpoint.FetchCached(s.Name())
		if err != nil {
			return
		}
	}

	ctx = new(ZohoScrapeContext)

	submatchStr := ZohoDomainArgsIdPattern.FindStringSubmatch(url)
	if len(submatchStr) >= 3 {
		ctx.Domain = submatchStr[1]
		ctx.ArgsId = submatchStr[2]
	} else {
		err = fmt.Errorf("%s: could not extract fields from pattern '%v'", s.Name(), ZohoDomainArgsIdPattern)
		return
	}

	var submatch [][]byte
	submatch = ZohoOwnerPattern.FindSubmatch(body)
	if submatch != nil {
		ctx.Owner = string(submatch[len(submatch)-1])
	} else {
		err = fmt.Errorf("%s: could not extract field from pattern '%v'", s.Name(), ZohoOwnerPattern)
		return
	}

	submatch = ZohoCsrfNamePattern.FindSubmatch(body)
	if submatch != nil {
		ctx.CsrfName = string(submatch[len(submatch)-1])
	} else {
		err = fmt.Errorf("%s: could not extract field from pattern '%v'", s.Name(), ZohoCsrfNamePattern)
		return
	}

	submatch = ZohoCsrfValuePattern.FindSubmatch(body)
	if submatch != nil {
		ctx.CsrfValue = string(submatch[len(submatch)-1])
	} else {
		err = fmt.Errorf("%s: could not extract field from pattern '%v'", s.Name(), ZohoCsrfValuePattern)
		return
	}

	ctx.Headers = []Header{
		Header{
			Name:  "Content-Type",
			Value: "application/x-www-form-urlencoded",
		},
		Header{
			Name:  "Cookie",
			Value: fmt.Sprintf("%s=%s", ctx.CsrfName, ctx.CsrfValue),
		},
		Header{
			Name:  "AGENT-TYPE",
			Value: "ZohoBookings",
		},
	}

	Log.Debugf("%s: domain: %s, args-id: %s, owner: %s, csrfName: %s, csrfValue: %s", s.Name(), ctx.Domain, ctx.ArgsId, ctx.Owner, ctx.CsrfName, ctx.CsrfValue)
	return
}

type ZohoExecuteRespWrapper struct {
	ReturnValue string `json:"returnvalue"`
}

type ZohoExecuteResp struct {
	BusinessId string   `json:"BUSINESS_ID"`
	ServiceIds []string `json:"SERVICE_IDS"`
}

type ZohoServicingResp struct {
	Data []ZohoServicingRespRecord `json:"data"`
}

type ZohoServicingRespRecord struct {
	Id            string          `json:"ID"`
	WorkspaceId   string          `json:"SERVICE_ID.WORKSPACE_ID"`
	StaffId       ZohoServicingId `json:"STAFF_ID"`
	ServiceId     ZohoServicingId `json:"SERVICE_ID"`
	ServiceStatus string          `json:"SERVICE_ID.SERVICE_STATUS"`
}

type ZohoServicingId struct {
	LinkRecId string `json:"linkrecid"`
	Value     string `json:"value"`
}

func (s *ScraperZoho) GetIds(ctx *ZohoScrapeContext) (body []byte, err error) {
	endpoint := new(Endpoint)
	endpoint.Url = fmt.Sprintf(ZohoExecuteUrl, ctx.Domain, ctx.Owner)
	endpoint.Method = "POST"
	endpoint.Body = fmt.Sprintf(ZohoExecuteBody, ctx.ArgsId, ctx.CsrfName, ctx.CsrfValue)
	endpoint.Headers = ctx.Headers
	body, _, err = endpoint.FetchCached(s.Name())
	if err != nil {
		return
	}

	respWrapper := new(ZohoExecuteRespWrapper)
	err = json.Unmarshal(body, respWrapper)
	if err != nil {
		return
	}

	resp := new(ZohoExecuteResp)
	err = json.Unmarshal([]byte(respWrapper.ReturnValue), resp)
	if err != nil {
		return
	}

	ctx.BusinessId = resp.BusinessId
	ctx.ServiceIds = resp.ServiceIds
	ctx.WorkspaceIds = make(map[string]string)
	ctx.ScheduleIds = make(map[string]string)

	endpoint = new(Endpoint)
	endpoint.Url = fmt.Sprintf(ZohoServicingUrl, ctx.Domain, ctx.Owner, ctx.Owner, strings.Join(ctx.ServiceIds, ","), ctx.CsrfName, ctx.CsrfValue)
	endpoint.Method = "GET"
	endpoint.Headers = ctx.Headers

	body, _, err = endpoint.FetchCached(s.Name())
	if err != nil {
		return
	}

	svcResp := new(ZohoServicingResp)
	err = json.Unmarshal(body, svcResp)
	if err != nil {
		return
	}

	badServiceIds := make(map[string]string)

	for _, record := range svcResp.Data {
		if record.ServiceId.LinkRecId != record.ServiceId.Value {
			Log.Warnf("%s: service id: linkrecid != value: %s != %s", s.Name(), record.ServiceId.LinkRecId, record.ServiceId.Value)
		}

		if record.ServiceStatus != "ACTIVE" {
			badServiceIds[record.ServiceId.Value] = record.ServiceStatus
			continue
		} else if !arrayContainsString(ctx.ServiceIds, record.ServiceId.Value) {
			badServiceIds[record.ServiceId.Value] = "HIDDEN"
			continue
		}

		if len(record.WorkspaceId) > 0 {
			ctx.WorkspaceIds[record.WorkspaceId] = record.ServiceId.Value
		} else {
			Log.Warnf("%s: workspace id for record %s is blank", s.Name(), record.Id)
			continue
		}

		if record.StaffId.LinkRecId != record.StaffId.Value {
			Log.Warnf("%s: staff id: linkrecid != value: %s != %s", s.Name(), record.StaffId.LinkRecId, record.StaffId.Value)
		}

		ctx.ScheduleIds[record.StaffId.Value] = record.WorkspaceId
	}

	if len(badServiceIds) > 0 {
		Log.Warnf("%s: service ids hidden or inactive: %v", s.Name(), badServiceIds)
	}

	Log.Debugf("%s: Business Id: %s, Service Ids: %v, Workspace Ids: [%s], Schedule Ids: [%s]",
		s.Name(), ctx.BusinessId, ctx.ServiceIds, mapKeysToStringList(ctx.WorkspaceIds, " "), mapKeysToStringList(ctx.ScheduleIds, " "))

	return
}

type ZohoSettingsResp struct {
	Data []ZohoSettingsRespRecord `json:"data"`
}

type ZohoSettingsRespRecord struct {
	Id          string `json:"ID"`
	ModelType   string `json:"MODEL_TYPE"`
	SettingId   string `json:"SETTING_ID"`
	SettingType string `json:"SETTING_KEY"`
	Value       string `json:"SETTING_VALUE"`
}

type ZohoBookingPref struct {
	TimeZone        string `json:"TIMEZONE"`
	BookingInterval int    `json:"SCHEDULING_INTERVAL"`
}

type ZohoSchedulePolicy struct {
	EnableCancel  bool `json:"ENABLE_CANCEL"`
	BookingStarts int  `json:"BOOKING_STARTS"`
	BookingEnds   int  `json:"BOOKING_ENDS"`
}

func (s *ScraperZoho) GetApptPrefs(ctx *ZohoScrapeContext) (body []byte, err error) {
	ctx.BusinessApptPrefs = NewZohoApptPrefs()
	ctx.WorkspaceApptPrefs = make(map[string]*ZohoApptPrefs)

	var idsStr string
	if len(ctx.WorkspaceIds) > 0 {
		idsStr = fmt.Sprintf("%s,%s", ctx.BusinessId, mapKeysToStringList(ctx.WorkspaceIds, ","))
	} else {
		idsStr = ctx.BusinessId
	}

	endpoint := new(Endpoint)
	endpoint.Url = fmt.Sprintf(ZohoPrefsUrl, ctx.Domain, ctx.Owner, ctx.Owner, idsStr, ctx.CsrfName, ctx.CsrfValue)
	endpoint.Method = "GET"
	endpoint.Headers = ctx.Headers

	body, _, err = endpoint.FetchCached(s.Name())
	if err != nil {
		return
	}

	settingsResp := new(ZohoSettingsResp)
	err = json.Unmarshal(body, settingsResp)
	if err != nil {
		return
	}

	for _, record := range settingsResp.Data {
		var apptPrefs *ZohoApptPrefs

		if record.ModelType == "BUSINESS" {
			apptPrefs = ctx.BusinessApptPrefs
		} else if record.ModelType == "WORKSPACE" {
			_, exists := ctx.WorkspaceApptPrefs[record.SettingId]
			if !exists {
				ctx.WorkspaceApptPrefs[record.SettingId] = NewZohoApptPrefs()
			}

			apptPrefs = ctx.WorkspaceApptPrefs[record.SettingId]
		} else {
			Log.Warnf("%s: unknown setting model type: %s", s.Name(), record.ModelType)
			continue
		}

		if apptPrefs == nil {
			Log.Warnf("%s: could not resolve %s id: %s", s.Name(), record.ModelType, record.SettingId)
			continue
		}

		if record.SettingType == "BOOKING_PREFERENCE" {
			Log.Debugf("%s: Found booking preference for %s id: %s", s.Name(), record.ModelType, record.SettingId)
			bookingPref := new(ZohoBookingPref)
			err = json.Unmarshal([]byte(record.Value), bookingPref)
			if err != nil {
				Log.Warnf("%v", err)
			} else {
				apptPrefs.Duration = bookingPref.BookingInterval
			}
		} else if record.SettingType == "SCHEDULING_POLICY" {
			Log.Debugf("%s: Found scheduling policy for %s id: %s", s.Name(), record.ModelType, record.SettingId)
			schedulingPolicy := new(ZohoSchedulePolicy)
			err = json.Unmarshal([]byte(record.Value), schedulingPolicy)
			if err != nil {
				Log.Warnf("%v", err)
			} else {
				apptPrefs.BookingStart = schedulingPolicy.BookingStarts
				apptPrefs.BookingEnd = schedulingPolicy.BookingEnds
			}
		} else {
			//ignore
			continue
		}
	}

	return
}

type ZohoScheduleResp struct {
	Data []ZohoScheduleRespRecord `json:"data"`
}

type ZohoScheduleRespRecord struct {
	Id         string `json:"ID"`
	ModelType  string `json:"MODEL_TYPE"`
	ScheduleId string `json:"SCHEDULE_ID"`
	From       string `json:"FROM"`
	To         string `json:"TO"`
	Data       string `json:"ADDITIONAL_ATTRIBUTES"`
}

type ZohoBlackoutResp struct {
	Data []ZohoBlackoutRespRecord `json:"data"`
}

type ZohoBlackoutRespRecord struct {
	Id         string `json:"ID"`
	Status     string `json:"STATUS"`
	ScheduleId string `json:"REFERENCE_ID"`
	From       string `json:"FROM_DATE_TIME"`
	To         string `json:"TO_DATE_TIME"`
}

type ZohoPeriod struct {
	Start time.Time
	End   time.Time
}

func (s *ScraperZoho) GetAvailability(ctx *ZohoScrapeContext) (available bool, body []byte, err error) {
	available = false

	now := time.Now().In(s.TimeZone)

	for workspaceId := range ctx.WorkspaceIds {
		scheduleIds := make([]string, 0)

		for scheduleId, workspaceId2 := range ctx.ScheduleIds {
			if workspaceId == workspaceId2 {
				scheduleIds = append(scheduleIds, scheduleId)
			}
		}

		var idsStr string
		if len(scheduleIds) > 0 {
			idsStr = fmt.Sprintf("%s,%s", strings.Join(scheduleIds, ","), ctx.BusinessId)
		} else {
			idsStr = ctx.BusinessId
		}

		apptPrefs := ctx.GetApptPrefs(workspaceId)

		if apptPrefs == nil {
			Log.Warnf("%s: Invalid appointment preferences: %s", s.Name(), workspaceId)
			continue
		}

		startTime := now.Add(time.Duration(apptPrefs.BookingStart) * time.Minute)
		start := strings.ReplaceAll(startTime.Format(ZohoTimeFormat), " ", "%20")
		endTime := now.Add(time.Duration(apptPrefs.BookingEnd) * time.Minute)
		end := strings.ReplaceAll(endTime.Format(ZohoTimeFormat), " ", "%20")

		endpoint := new(Endpoint)
		endpoint.Url = fmt.Sprintf(ZohoBlackoutUrl, ctx.Domain, ctx.Owner, ctx.Owner, idsStr, end, start, ctx.CsrfName, ctx.CsrfValue)
		endpoint.Method = "GET"
		endpoint.Headers = ctx.Headers
		body, _, err = endpoint.FetchCached(s.Name())
		if err != nil {
			return
		}

		blackoutResp := new(ZohoBlackoutResp)
		err = json.Unmarshal(body, blackoutResp)
		if err != nil {
			return
		}

		blackoutPeriods := make(map[string][]ZohoPeriod)
		for _, blackoutRecord := range blackoutResp.Data {
			if blackoutRecord.Status != "PLANNED" {
				continue
			}

			if _, exists := blackoutPeriods[blackoutRecord.ScheduleId]; !exists {
				blackoutPeriods[blackoutRecord.ScheduleId] = make([]ZohoPeriod, 0)
			}

			start, parseErr := time.ParseInLocation(ZohoTimeFormat, blackoutRecord.From, s.TimeZone)
			if parseErr != nil {
				Log.Errorf("%s: %v", s.Name(), parseErr)
			}

			end, parseErr := time.ParseInLocation(ZohoTimeFormat, blackoutRecord.To, s.TimeZone)
			if parseErr != nil {
				Log.Errorf("%s: %v", s.Name(), parseErr)
			}

			blackoutPeriods[blackoutRecord.ScheduleId] = append(blackoutPeriods[blackoutRecord.ScheduleId], ZohoPeriod{
				Start: start,
				End:   end,
			})
		}

		endpoint = new(Endpoint)
		endpoint.Url = fmt.Sprintf(ZohoScheduleUrl, ctx.Domain, ctx.Owner, ctx.Owner, idsStr, end, start, ctx.CsrfName, ctx.CsrfValue)
		endpoint.Method = "GET"
		endpoint.Headers = ctx.Headers

		body, _, err = endpoint.FetchCached(s.Name())
		if err != nil {
			return
		}

		scheduleResp := new(ZohoScheduleResp)
		err = json.Unmarshal(body, scheduleResp)
		if err != nil {
			return
		}

		for _, scheduleId := range scheduleIds {
			var slotConf *ZohoSlotConf
			var apptsList []*ZohoAppointments

			Log.Debugf("%s: checking schedule id %s", s.Name(), scheduleId)

			for _, record := range scheduleResp.Data {
				if record.ScheduleId == scheduleId {
					if record.ModelType == "WORKINGHRS_SLOTS" {
						slotConf, err = GetZohoSlotConf(&record)
						if err != nil {
							Log.Warnf("%v", err)
						}
					} else if record.ModelType == "APPOINTMENT_SLOTS" {
						appts, err := GetZohoAppointments(&record, s.TimeZone)
						if err != nil {
							Log.Warnf("%v", err)
						} else {
							apptsList = append(apptsList, appts)
						}
					} else {
						Log.Warnf("%s: Unknown record type for schedule id %s: %s", s.Name(), scheduleId, record.ModelType)
					}
				}
			}

			if len(apptsList) == 0 {
				Log.Debugf("%s: No appointment data for schedule id %s", s.Name(), scheduleId)
				continue
			}

			if slotConf == nil {
				Log.Errorf("%s: schedule id %s has appointments but no hours configuration", s.Name(), scheduleId)
				continue
			}

			if firstAvail := GetFirstAvailability(slotConf, apptsList, apptPrefs.Duration, startTime, blackoutPeriods[scheduleId]); !firstAvail.IsZero() && firstAvail.Before(endTime) {
				available = true
				Log.Debugf("%s: Availability found in schedule id %s: %v", s.Name(), scheduleId, firstAvail)
				return
			}

			Log.Debugf("%s: No availability found in schedule id %s", s.Name(), scheduleId)
		}
	}

	return
}

type ZohoAppointments struct {
	From   time.Time
	To     time.Time
	Bitmap [288]bool
}

func GetZohoAppointments(rec *ZohoScheduleRespRecord, timeZone *time.Location) (appts *ZohoAppointments, err error) {
	if rec.ModelType != "APPOINTMENT_SLOTS" {
		return nil, fmt.Errorf("Expecting model type 'APPOINTMENT_SLOTS', got '%s'", rec.ModelType)
	}

	appts = new(ZohoAppointments)
	appts.From, err = time.ParseInLocation(ZohoTimeFormat, rec.From, timeZone)
	if err != nil {
		return nil, err
	}
	appts.To, err = time.ParseInLocation(ZohoTimeFormat, rec.To, timeZone)
	if err != nil {
		return nil, err
	}

	if !regexp.MustCompile("^[01]{288}$").MatchString(rec.Data) {
		return nil, fmt.Errorf("Expecting bit string of length 288, got '%s'", rec.Data)
	}

	for idx, c := range rec.Data {
		if c == '1' {
			appts.Bitmap[idx] = true
		} else {
			appts.Bitmap[idx] = false
		}
	}

	return
}

type ZohoSlotConf struct {
	Days [7]ZohoSlotConfDay
}

type ZohoSlotConfDay struct {
	Enabled bool
	Start   int //minutes past midnight
	End     int
}

type ZohoTimingData struct {
	Enabled bool                `json:"isEnable"`
	Days    []ZohoTimingDataDay `json:"timing"`
}

type ZohoTimingDataDay struct {
	Enabled  bool                     `json:"IS_ENABLE"`
	DayIndex int                      `json:"DAY_INDEX"`
	Time     [1]ZohoTimingDataDayTime `json:"WEEK_SCHEDULE_TIMINGS"`
}

type ZohoTimingDataDayTime struct {
	From string `json:"FROM"`
	To   string `json:"TO"`
}

func GetZohoSlotConf(rec *ZohoScheduleRespRecord) (slotConf *ZohoSlotConf, err error) {
	if rec.ModelType != "WORKINGHRS_SLOTS" {
		return nil, fmt.Errorf("Expecting model type 'WORKINGHRS_SLOTS', got '%s'", rec.ModelType)
	}

	slotConf = new(ZohoSlotConf)

	timingData := new(ZohoTimingData)

	err = json.Unmarshal([]byte(rec.Data), timingData)
	if err != nil {
		return nil, err
	}

	referenceTime, _ := time.Parse(ZohoTimeFormatSlot, "01-Jan-1970 12:00 am")

	for dayIndex := 0; dayIndex < 7; dayIndex++ {
		found := false
		for _, day := range timingData.Days {
			if day.DayIndex == dayIndex {
				found = true
				slotConf.Days[dayIndex].Enabled = timingData.Enabled && day.Enabled
				startTimeTmp, err := time.Parse(ZohoTimeFormatSlot, fmt.Sprintf("01-Jan-1970 %s", day.Time[0].From))
				if err != nil {
					return nil, err
				}
				endTimeTmp, err := time.Parse(ZohoTimeFormatSlot, fmt.Sprintf("01-Jan-1970 %s", day.Time[0].To))
				if err != nil {
					return nil, err
				}

				//convert to number of minutes past midnight
				slotConf.Days[dayIndex].Start = int(math.Ceil(startTimeTmp.Sub(referenceTime).Minutes()))
				slotConf.Days[dayIndex].End = int(math.Floor(endTimeTmp.Sub(referenceTime).Minutes()))

				if slotConf.Days[dayIndex].Start < 0 {
					return nil, fmt.Errorf("Working hours start time less than 0!")
				} else if slotConf.Days[dayIndex].End >= 24*60 {
					return nil, fmt.Errorf("Working hours end time more than %d (past midnight)!", 24*60)
				}
			}
		}

		if !found {
			return nil, fmt.Errorf("Day index %d not found!", dayIndex)
		}
	}

	return slotConf, nil
}

func GetFirstAvailability(workingHours *ZohoSlotConf, apptsList []*ZohoAppointments, apptDuration int, startTime time.Time, blackoutPeriods []ZohoPeriod) time.Time {

	//round up to nearest 5 minute increment
	apptDuration5min := int(math.Ceil(float64(apptDuration) / 5.0))

	for _, appts := range apptsList {
		for idx := 0; idx <= 288-apptDuration5min; idx += apptDuration5min {
			apptRealTime := appts.From.Add(time.Duration(idx) * 5 * time.Minute)

			//day of week
			apptDayIndex := apptRealTime.Weekday()

			//number of minutes past midnight
			apptDayTime := apptRealTime.Hour()*60 + apptRealTime.Minute()

			if !workingHours.Days[apptDayIndex].Enabled {
				//closed on this day
				continue
			}

			if apptDayTime < workingHours.Days[apptDayIndex].Start || (apptDayTime+apptDuration) > workingHours.Days[apptDayIndex].End {
				//outside of working hours
				continue
			}

			booked := false
			for j := idx; j < idx+apptDuration5min; j++ {
				if appts.Bitmap[j] {
					booked = true
					break
				}
			}

			if booked {
				continue
			}

			if apptRealTime.Before(startTime) {
				continue
			}

			apptStart := apptRealTime
			apptEnd := apptRealTime.Add(time.Duration(apptDuration5min) * 5 * time.Minute)

			inBlackout := false
			for _, period := range blackoutPeriods {
				if !apptStart.Before(period.Start) && apptStart.Before(period.End) {
					//appt start inside period
					inBlackout = true
					break
				}

				if apptEnd.After(period.Start) && !apptEnd.After(period.End) {
					//appt end inside period
					inBlackout = true
					break
				}

				if apptStart.Before(period.Start) && apptEnd.After(period.End) {
					//appt encompasses period
					inBlackout = true
					break
				}
			}

			if inBlackout {
				continue
			}

			Log.Debugf("Appointment found at index %d, minute %d, working %d - %d", idx, apptDayTime, workingHours.Days[apptDayIndex].Start, workingHours.Days[apptDayIndex].End)

			return apptRealTime
		}
	}

	var zeroTime time.Time
	return zeroTime
}

func arrayContainsString(arr []string, str string) bool {
	for _, v := range arr {
		if str == v {
			return true
		}
	}

	return false
}

func mapKeysToStringList(m map[string]string, delimiter string) string {
	sb := new(strings.Builder)

	for v := range m {
		if sb.Len() > 0 {
			sb.WriteString(delimiter)
		}
		sb.WriteString(v)
	}

	return sb.String()
}
