#!/usr/bin/env python

"""
    Scraper for Cascade Medical.

    As of 3/8/21, their site is reporting No Appointments Available. This scraper may
    need to be updated once they do have appointments available. We are making an attempt,
    here, to look at availability of 1st doses and 2nd doses if the failure case is not met.
"""
import requests
from bs4 import BeautifulSoup
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable
from typing import Tuple, List, Optional
import logging
from time import sleep

class CascadeMedical(ScraperBase):

    def __init__(self):
        # The main page url
        self.URL = "https://cascademedical.org/keeping-you-informed-covid-19"
        self.Timeout: int = 3
        self.WaitListCase = ""
        self.FailureCase = "no"
        self.LocationName = "Cascade Medical, Leavenworth, WA"
        self.Keys = ["cascade_medical"]

    @SaveHtmlToTable
    def MakeGetRequest(self) -> Tuple[List[str], Status, Optional[str]]:

        # Start with status UNKNOWN, because GET to this site intermittently gets
        # a 'Connection reset by peer' error. Status doesn't change to POSSIBLE
        # until the GET succeeds.
        case = Status.UNKNOWN
        try:
            r = requests.get(self.URL, timeout=self.Timeout)
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

        # Everything starts out as POSSIBLE.
        case = Status.POSSIBLE

        # Search for the Failure Case deep in the html.
        # Got the xpath from Chrome Inspect, converted
        # for soup.select(). IS THERE A BETTER WAY?
        # xpath: //*[@id="block-covidvaccinetracker2"]/div[2]/div[2]/div/h2/a/div

        result = soup.select("#block-covidvaccinetracker2 > div:nth-of-type(2) > div:nth-of-type(2) > div > h2 > a > div ")
        if result:
            appts_avail = result[0].text
            if self.FailureCase in appts_avail.lower():
                logging.info(f"No Appointments Available")
                case = Status.NO
            else:
                # We weren't told there are no appointments available, so
                # now try finding 1st doses available for distribution.
                #
                # xpath: //*[@id="block-covidvaccinetracker2"]/div[5]/div[2]
                dose1_avail = 0
                dose2_avail = 0

                result = soup.select("#block-covidvaccinetracker2 > div:nth-of-type(5) > div:nth-of-type(2)")
                if result:
                    dose1_avail = int(result[0].text)
                    # print(f"1st Doses Available: {dose1_avail}")
                else:
                    logging.info(self.LocationName + " xpath to dose1 availability changed, recheck")

                # Now try finding 2nd doses available for distribution.
                #
                # xpath: //*[@id="block-covidvaccinetracker2"]/div[6]/div[2]
                result = soup.select("#block-covidvaccinetracker2 > div:nth-of-type(6) > div:nth-of-type(2)")
                if result:
                    dose2_avail = int(result[0].text)
                    #print(f"2nd Doses Available: {dose2_avail}")
                else:
                    logging.info(self.LocationName + " xpath to dose2 availability changed, recheck")

                # Not sure we are tracking availability by dose type yet, so
                # right now, return YES if either type is available.
                if (dose1_avail + dose2_avail > 0):
                    logging.info(f"Number of 1st + 2nd doses available: {dose1_avail + dose2_avail}")
                    case = Status.YES

        if case == Status.POSSIBLE:
            # We don't have a NO or a YES. Maybe something changed on the site.
            logging.info(self.LocationName + " site has changed, recheck")

        return self.Keys, case, r.text

if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)

    scraper = CascadeMedical()
    keys, case, text = scraper.MakeGetRequest()

    logging.debug(f"keys={keys} case={case}")
