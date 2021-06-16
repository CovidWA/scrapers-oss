"""

This scraper is for sites using Acuity Scheduling, including
the Family Care Network in Whatcom and Skagit County and Kusler's
Pharmacy. Each location, their keys, and their API calls are stored in
AcuityScheduling.csv. The AcuitySchedulingWrapper loads the data from
the csv file and the MakeAllRequests method creates an AcuityScheduling
class for each location.

An API is called in the AcuityScheduling class,
which returns the calendar of available appointments. The API return is
parsed for the known failure case and for the success case (appointments are
available). The site for FCN currently has separate appointments
for Moderna and Johnson & Johnson. Currently only the Moderna
appointments are scraped. Several locations are listed as "Temporarily
unavailable" or "coming soon". All of these locations are also scraped.

N.B. Depending on what the differentiator turns out to be (Type? Calendar? CalendarID?),
     we may be able to modify the scraper to do multiple API calls (one per
     vaccine type) by changing the associated differentiator in csv into
     a list, rather than the single value it is now.

The columns in the csv file are as follows:
    Location - name from airtable, used in logging
    ApiURL - the URL used for the POST request to the API endpoint
    Owner, Template (usually weekly), Type, Calendar, NumDays (usually 3 or 5),
        and CalendarID (often null) come from Chrome->Inspect->Network/XHR: select
        the API request, it is in the Name section and starts with 'schedule.php?',
        then look at the bottom of the Headers to pick the fields out of the
        Query String Parameters and the Form Data.
    Key - key from airtable
    URL - the site's normal URL (not the API URL), we make a GET request to URL
        so we can return the response text.

"""

import requests
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable, LIMITED_THRESHOLD
import logging
import csv


class AcuitySchedulingWrapper():
    '''wrapper function for Acuity Scheduling locations. Instantiates an
    instance of the AcuityScheduling class for each location'''
    def __init__(self):
        '''initialize the AcuitySchedulingWrapper class and call a method
        to load the csv with different location data'''

        self.Locations = []        # list of dictionaries holding loc info

        # Columns from AcuityScheduling.csv - these should match the CSV.
        # Location is used for logging. URL is only used for the GET request
        # so that r.text can be returned. The important columns are ApiURL, Owner,
        # Type, Calendar NumDays, and sometimes CalendarId is non-null.
        # Key is the key in airtable.

        self.columns = ["Location", "ApiURL", "Owner", "Template", "Type", "Calendar", \
                        "NumDays", "CalendarId", "Key", "URL"]

        self.ReadLocations()       # load the file of locations

    def ReadLocations(self):
        '''open the csv storing location data and read into self.Locations'''

        with open("AcuityScheduling.csv") as csvfile:
            locReader = csv.reader(csvfile)

            for row in locReader:
                locationDic = {}   # dictionary holding data for each location

                # first row has the column headers - skip this row
                if row[0] == self.columns[0]:
                    continue

                # add columns to dictionary
                for i, col in enumerate(self.columns):
                    locationDic[col] = row[i]
                # add locations dic to the list
                self.Locations.append(locationDic)
        return

    def MakeGetRequest(self):
        '''create a AcuityScheduling scraper class for each location and
        call method to scrape location and update air table'''

        cols = self.columns

        # iterate through all the locations
        for loc in self.Locations:
            scraper = AcuityScheduling(loc)
            keys, case, text = scraper.MakeGetRequest()
            logging.debug(f"keys={keys} case={case}")

        return

class AcuityScheduling(ScraperBase):
    '''class for a scraper that looks at a single location in the FCN
    network'''

    def __init__(self, locDict):
        '''initialization method. Takes a locDict dictionary with information
        about a single location. The keys of the location dictionary MUST
        match the self.columns attribute of the AcuityScheduling wrapper'''
        # template for API url and success/failure patterns
        self.ApiURL = locDict["ApiURL"]             # URL of the curl API call
        self.Success = ["select-day", "activeCalendarDay"]  # success cases in API return
        self.ApptStr = "<input type="               # string for single appt
        self.Failure = 'No times are available'     # failure case in API return

        self.URL = locDict["URL"]                   # site URL - used only to return response text
        self.LocationName = locDict["Location"]     # location name - used only for logging
        self.Keys = [locDict["Key"]]                # location key

        # Params is the params sent in the requests.POST
        params = "action=showCalendar&fulldate=1&owner=__OWNER__&template=__TEMPLATE__"
        self.Params = params.replace('__OWNER__', locDict["Owner"]).replace('__TEMPLATE__', locDict["Template"])

        # DataDict is the data sent in the requets.POST
        self.DataDict = {'type': '', 'calendar': '', 'skip': 'true', 'options[qty]': '1', 'options[numDays]': '', \
                    'ignoreAppointment': '', 'appointmentType': '', 'calendarID': ''}
        self.DataDict['type'] = locDict["Type"]
        self.DataDict['calendar'] = locDict['Calendar']
        self.DataDict['options[numDays]'] = locDict["NumDays"]
        self.DataDict['calendarID'] = locDict['CalendarId']     # calendarID is often ''

        return

    @SaveHtmlToTable
    def MakeGetRequest(self):
        # Make outbound GET to the URL in question - URL not actually used
        # except to return r.text
        case = Status.UNKNOWN
        try:
            r = requests.get(self.URL)
        except requests.exceptions.RequestException as err:
            logging.debug(f"request exception {err}")
            return self.Keys, case, None

        # Response overloads bool, so r is True for status_codes 200 - 400,
        # otherwise False.
        if not r:
            logging.debug(f"GET Response Failed with {r.status_code}")
            return self.Keys, case, None

        # call the API
        try:
            ApiReturn = requests.post(
                self.ApiURL,
                params=self.Params,
                data=self.DataDict
                )
        except requests.exceptions.RequestException as err:
            logging.debug(f"POST request exception {err}")
            return self.Keys, case, None

        case = Status.POSSIBLE      # assume possible to start

        # Are times  available?
        if self.Success[0] in ApiReturn.text:
            numAppts = ApiReturn.text.count(self.ApptStr)
            if numAppts > LIMITED_THRESHOLD:
                case = Status.YES
            else:
                case = Status.LIMITED
        # alternative success string found 
        elif self.Success[1] in ApiReturn.text:
            case = Status.YES

        # failure string found denotes no slots
        elif self.Failure in ApiReturn.text:
            case = Status.NO

        if case == Status.POSSIBLE:
            # Failure case not met, leave as possible
            # HTML will be auto uploaded by wrapper function
            logging.info(self.LocationName + " site has changed, recheck")

        return self.Keys, case, r.text

if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)
    scraper = AcuitySchedulingWrapper()
    scraper.MakeGetRequest()
