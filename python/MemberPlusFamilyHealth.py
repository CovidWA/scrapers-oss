#!/usr/bin/env python

import requests
from bs4 import BeautifulSoup
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable
import logging
from typing import Tuple, List, Optional

class MemberPlusFamilyHealth(ScraperBase):

    def __init__(self):
        # The main page url
        self.URL = "https://www.memberplusfamilyhealth.com"
        self.alternateURL = "https://vaccinempfh.timetap.com/"
        self.LocationName = "MEMBER PLUS FAMILY HEALTH, Bainbridge Island, WA"
        self.Keys = ["member_plus_family_health"]

        self.FailureCase = "appointments are filled"
        self.WaitListCase = "already on our list"

    @SaveHtmlToTable
    def MakeGetRequest(self) -> Tuple[List[str], Status, Optional[str]]:

        case = Status.UNKNOWN   # Start as Unknown in case the GET fails

        # Make outbound GET to the URL in question
        try:
            r = requests.get(self.URL, timeout=5)    # Sometimes doesn't work without timeout
        except requests.exceptions.RequestException as err:
            logging.debug(f"request exception {err}")
            return self.Keys, case, None

        # Response overloads bool, so r is True for status_codes 200 - 400,
        # otherwise False.
        if not r:
            logging.debug(f"Response Failed with {r.status_code}")
            return self.Keys, case, None

        # Parse the HTML
        soup = BeautifulSoup(r.content, 'html.parser')

        # Search through elements in the DOM now
        listItems = soup.find_all('p')

        # From here on, everything begins as POSSIBLE
        case = Status.POSSIBLE

        # Try to scrape website for Waitlist and Failure cases.
        for element in listItems:
            if self.WaitListCase in element.text.lower():
                logging.info(self.LocationName + ". Existing patients are already on the list, no need to call")
                case = Status.WAITLIST
                break
            if self.FailureCase in element.text.lower():
                logging.info(self.LocationName + ". CURRENT APPOINTMENTS ARE FILLED, BUT WE WILL UPDATE THIS WEBPAGE IF MORE APPOINTMENT SLOTS BECOME AVAILABLE")
                case = Status.NO
                break

        # BUGBUG: Did not find Waitlist or Failure case, so now try to find availability
        # using the Timetap API. Code still TBD.

        if case == Status.POSSIBLE:
            # Leave as possible, HTML will be auto uploaded by wrapper function
            logging.info(self.LocationName + " site has changed, recheck")

        return self.Keys, case, r.text


if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)
    scraper = MemberPlusFamilyHealth()
    keys, case, text = scraper.MakeGetRequest()
    logging.debug(f"keys={keys} case={case}")
