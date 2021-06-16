#!/usr/bin/env python
"""
    Grays Harbor County Scraper. They use Acuity Scheduling, but you have to get
    the link off their page first. It's not known in advance, and it changes
    regularly. If they've removed the link, that indicates no availability.
    If the link is there, there may or may not be availability. Uses the
    Acuity Scheduling API. While the link may change, it currently appears that
    owner = 21734180, and type = 19665945, are constant. CalendarID changes,
    so we need to pick it off by following the ApiURL and grabbing it from there.
    Default to a random number, in case we don't find any calendarID.
"""
import requests
from bs4 import BeautifulSoup
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable, LIMITED_THRESHOLD
import logging
from typing import Tuple, List, Optional
import re


class GraysHarborCounty(ScraperBase):

    def __init__(self):
        """Initialize data for Grays Harbor."""
        self.URL = "https://www.healthygh.org/covid19-vaccine-appointment"
        self.LocationName = (
            "Grays Harbor County Public Health & Social Services"
        )
        self.Keys: List[str] = ["grays_harbor_county"]
        self.ApiURL: List[str] = []        # List of URLs for the API call, get from soup
        self.Success = "select-day"        # Success case in API return
        self.ApptStr = "<input type="      # String for single appt

        # Params is the params sent in the requests.POST. Acquired via Chrome->Inspect.
        self.Params = "action=showCalendar&fulldate=1&owner=21734180&template=weekly"

        # DataDict is the data sent in the requets.POST. Acquired via Chrome->Inspect.
        self.DataDict = {'type': '19665945', 'calendar': '5555555', 'skip': 'true', 'options[qty]': '1', 'options[numDays]': '5', \
                            'ignoreAppointment': '', 'appointmentType': '', 'calendarID': '5555555'}

    @SaveHtmlToTable
    def MakeGetRequest(self) -> Tuple[List[str], Status, Optional[str]]:
        """Request data from Grays Harbor via Acuity Scheduling API."""

        # Everything begins as UNKNOWN, until the GET request succeeds.
        case = Status.UNKNOWN

        # Make outbound GET to the page where the ApiURL resides.
        try:
            r = requests.get(self.URL, timeout=3)
        except requests.exceptions.RequestException as err:
            logging.debug(f"request exception {err}")
            return self.Keys, case, None

        if not r:
            logging.debug(f"Response Failed with {r.status_code}")
            return self.Keys, case, None

        # Parse the HTML.
        soup = BeautifulSoup(r.content, "html.parser")

        # Pick out the ApiURL link for Acuity Scheduling
        self.ApiURL = [link.get('href') for link in soup.select("a[href*='//ph-graysharbor.as.me']")]

        if not self.ApiURL:
            # They removed the link because there is no availability.
            logging.debug(f"Did not find link to Acuity Scheduling")
            case = Status.NO
            return self.Keys, case, r.text

        # Now go to the ApiURL page and try to pick off calendarID to
        # use in the API call.

        for url in self.ApiURL:
            try:
                r = requests.get(url, timeout=3)
            except requests.exceptions.RequestException as err:
                logging.debug(f"request exception for {url}: {err}")
                return self.Keys, case, None

            if not r:
                logging.debug(f"Response Failed with {r.status_code} for {url}")
                return self.Keys, case, None

            # Parse the HTML.
            soup = BeautifulSoup(r.content, "html.parser")

            # Pick out the calendarID from the soup.
            calItems = soup.select("a[href*='/schedule.php?owner=21734180']")

            if calItems:
                stuff, cal = calItems[0].get('href').rsplit('=', 1)

            self.DataDict['calendar'] = self.DataDict['calendarID'] = cal

            # Call the API
            try:
                ApiReturn = requests.post(
                    url,
                    params=self.Params,
                    data=self.DataDict
                    )
            except requests.exceptions.RequestException as err:
                logging.debug(f"API POST request exception for {url}: {err}")
                return self.Keys, case, None

            # From here on, assume POSSIBLE
            case = Status.POSSIBLE

            # Are times available?
            if self.Success in ApiReturn.text:
                # Check for number of appointments to differentiate YES v. LIMITED.
                # If YES, break out of the loop. If LIMITED, continue checking in
                # case we turn up a YES. Do not overwrite a LIMITED with a NO, though.
                numAppts = ApiReturn.text.count(self.ApptStr)
                logging.info(f"Appt count = {numAppts} for {url}")
                if numAppts > LIMITED_THRESHOLD:
                    case = Status.YES
                    break
                else:
                    case = Status.LIMITED
                    continue

            # Failure string found denotes no slots
            elif re.match(r'no (appointment|times)(.*)available(.*)', ApiReturn.text, re.IGNORECASE) and \
                            (case != Status.LIMITED) :
                case = Status.NO

        if case == Status.POSSIBLE:
            # Failure case not met, leave as POSSIBLE.
            # HTML will be auto uploaded by wrapper function.
            logging.info(self.LocationName + " site has changed, recheck")

        logging.info(f"Returning {case} for {url}")
        return self.Keys, case, r.text

if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)

    scraper = GraysHarborCounty()
    keys, case, text = scraper.MakeGetRequest()
    logging.debug(f"keys={keys} case={case}")
