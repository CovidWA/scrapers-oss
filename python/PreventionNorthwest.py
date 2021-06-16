#!/usr/bin/env python

"""
    Prevention Northwest is a mobile vaccination service located in Spokane. They have some
    sort of affiliation with Northwest Neurological, but the latter is not doing
    vaccinations. Prevention Northwest recently changed from No Info to
    Waitlist. Additions to the Waitlist are via email to Covidinfo@preventionnw.com.

    N.B. This clinic's site often gets 'Connection reset by peer' on the first GET,
         so we retry once before failing.
"""
import requests
from bs4 import BeautifulSoup
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable
from typing import Tuple, List, Optional
import logging
from time import sleep


class PreventionNorthwest(ScraperBase):

    def __init__(self):
        # The main page url
        self.URL = "https://preventionnw.com/services"
        self.Timeout: int = 3
        self.WaitListCase = "to be added to our waiting list"
        self.LocationName = "Prevention Northwest, Spokane, WA"
        self.Keys = ["prevention_northwest"]

    @SaveHtmlToTable
    def MakeGetRequest(self) -> Tuple[List[str], Status, Optional[str]]:

        # Start with status UNKNOWN, because GET to this site intermittently gets
        # a 'Connection reset by peer' error. Status doesn't change to POSSIBLE
        # until the GET succeeds.
        case = Status.UNKNOWN
        retry_count = 1
        while True:
            try:
                r = requests.get(self.URL, timeout=self.Timeout)
                break
            except requests.exceptions.RequestException as err:
                    # Retry one time, as this site tends to get an exception on
                    # the first request, but succeeds ther
                    if retry_count < 2:
                        logging.debug(f"First catch of exception {err}, retrying")
                        sleep(0.5)
                        retry_count += 1
                        continue
                    else:
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
        listItems = soup.find_all('li')

        # Everything begins as possible (maybe changes in V2)
        case = Status.POSSIBLE

        for element in listItems:
            if self.WaitListCase in element.text:
                # This is the WAITLIST or CALL/PHONE/EMAIL (V2?) case
                logging.info(self.LocationName + ". Email Covidinfo@preventionnw.com to add to waitlist")
                case = Status.WAITLIST
                break

        if case == Status.POSSIBLE:
            # Failure case (not currently applicable) not met, Waitlist case not met, leave as possible,
            # HTML will be auto uploaded by wrapper function
            logging.info(self.LocationName + " site has changed, recheck")

        return self.Keys, case, r.text


if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)

    scraper = PreventionNorthwest()
    keys, case, text = scraper.MakeGetRequest()

    logging.debug(f"keys={keys} case={case}")
