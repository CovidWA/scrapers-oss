# updated to use requests library instead of curl command
#
# The Yakima Valley Health provider website formerly had several "lines"
# corresponding to different API calls. The site has since changed so that
# there is a single line and a single API called for calendar booking.
# The script is still written to iterate over the lines (though there is now
# only a single line) in case the site switches again to multiple lines

import requests
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable
import logging
from typing import Tuple, List
import time

class YakimaValley(ScraperBase):

    def __init__(self):

        # template for API url and success/failure patterns
        self.ApiURL = 'https://www.clockwisemd.com/hospitals/6717/appointments/available_times?page_type=urgent_care_online_time_by_reason&version=v1&reason_description=COVID%20Vaccine%201st%20Dose%20Line%20<LINE>&_=<UNIX_TIME>'
        self.APISuccess = "display_time"                 # presence means yes

        # url and success/failure case of the sites - the API results instead
        # of these success/failure cases are used
        self.URL = "https://www.yakimamemorial.org/medical-services-covid-vaccinations.asp"
        self.SuccessCase = "Please select one of the following lines for a FIRST dose"
        self.FailureCase = "We have exhausted our supply of vaccine and have no appointment availability for first doses"
        self.LocationName = "YakimaValley"
        self.Keys = ["yakima_valley_new"]
        self.NumLines = 1   # number of "lines" on the provider site

    def checkAppointments(self, lineIndex: int) -> Status:
        '''calls the scheduling API to see if there are times available
           args: lineIndex - index of the "line" corresponding to site
                             dropdown menu'''

        # build the appropriate Api URL
        apiCommand = self.ApiURL.replace("<UNIX_TIME>", str(int(time.time())))
        apiCommand = apiCommand.replace("<LINE>", str(lineIndex))

        r = requests.get(apiCommand)        # call the API

        # convert to string but catch if json conversion fails
        try:
            outDict = dict(r.json())       # convert response to dict
            outString = str(outDict)       # convert JSON to string
        except:
            logging.info("Yakima failed to parse json - site may have changed")
            return Status.POSSIBLE

        # times are available
        if self.APISuccess in outString:
            logging.info("Yakima found appointment line: {}".format(lineIndex))
            return Status.YES

        empty = 0              # count number of empty elements
        # iterate through the keys and see if the value is an empty list
        for key in outDict:
            if outDict[key] == []:
                empty += 1

        # all the elements are empty - return no appointment
        if empty == len(outDict):
           logging.info("Yakima not found appointment line: {}".format(lineIndex))
           return Status.NO

        # string not recognized - result unknown
        return Status.POSSIBLE

    @SaveHtmlToTable
    def MakeGetRequest(self)-> Tuple[List[str], Status, str]:
        # Make outbound GET to the URL in question - URL not actually used
        r = requests.get(self.URL)

        # dict for counting each status
        statusCount = {Status.POSSIBLE:0, Status.NO: 0, Status.YES: 0}

        # assume possible and iterate through the "lines" on provider site
        case = Status.POSSIBLE
        for i in range(self.NumLines):
            # calculate "line" index and increment the lines w/ returned status
            lineIndex = i + 1
            status = self.checkAppointments(lineIndex)
            statusCount[status] += 1

            # found a line w/ appointments, return yes
            if status == Status.YES:
                case = Status.YES
                break

        # all lines have confirmed fail
        if statusCount[Status.NO] == self.NumLines:
            case = Status.NO

        if case == Status.POSSIBLE:
            # Failure case not met, leave as possible
            # HTML will be auto uploaded by wrapper function
            logging.info(self.LocationName + " site has changed, recheck")

        return self.Keys, case, r.text

if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)
    scraper = YakimaValley()
    keys, case, text = scraper.MakeGetRequest()
    logging.debug(f"keys={keys} case={case}")
