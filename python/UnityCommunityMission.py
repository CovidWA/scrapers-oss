#!/usr/bin/env python

import requests
from bs4 import BeautifulSoup
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable
import logging


class UnityCommunityMission(ScraperBase):
    def __init__(self):
        # The main page url
        # self.URL = "https://www.yvfwc.com/covid-19-vaccine-newsletter-2/"
        self.URL = "https://www.yvfwc.com/locations/unify-community-health-mission/"        # string
        self.FailureCase = "No information"
        self.LocationName = "Unity Community Health at Mission, Spokane"
        self.Keys = ["unity_community_mission"]

    @SaveHtmlToTable
    def MakeGetRequest(self):
        # Make outbound GET to the URL in question
        r = requests.get(self.URL)

        # Parse the HTML
        soup = BeautifulSoup(r.content, "html.parser")

        # Search through elements in the DOM now
        # listItems = soup.find_all('span')
        listItems = soup.find_all("h4")
        # Everything begins as possible, unless tagged as a NO

        case = Status.UNKNOWN
        for element in listItems:
            if self.FailureCase in element.text:
                # Must call to find out
                logging.info(self.LocationName + " no information")
                # case = Status.CALL if this were possible otherwise WAITLIST
                case = Status.WAITLIST
                break

        # this really should be the call case
        if case == Status.POSSIBLE:
            # Failure case not met, leave as possible, HTML will be auto uploaded by wrapper function
            logging.info(self.LocationName + " site has changed, recheck")

        return self.Keys, case, r.text


if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)

    scraper = UnityCommunityMission()
    keys, case, text = scraper.MakeGetRequest()

    logging.debug(f"keys={keys} case={case}")
