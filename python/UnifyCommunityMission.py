#!/usr/bin/env python
"""
Scraper for Unify Community Mission.

As of 2/26/21: Must call or email to get added to their waitlist. So as of now,
this scraper will just return WAITLIST (may change in the future V2), even if there
are exceptions from requests.get(). Note that this site returns 403 Forbidden
when accessed from a scaper (rather than via a browser), so we handle that.

    Call: 509-326-4343
    Email: unifycommunityhealth@yvfwc.org, and provide the following information:
        Name, Address, Date of Birth, How you qualify for vaccination in current phase

"""
import requests
from bs4 import BeautifulSoup
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable
import logging
from typing import Tuple, List, Optional

class UnifyCommunityMission(ScraperBase):
    def __init__(self):
        # The main page url
        self.URL = "https://www.yvfwc.com/locations/unify-community-health-mission/"
        # We need User-Agent, otherwise the scraper gets 403 Forbidden
        self.headers = {'User-Agent': "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/88.0.4324.150 Safari/537.36"}     # string
        self.FailureCase = "No information"
        self.LocationName = "Unify Community Health at Mission, Spokane"
        self.Keys = ["unify_community_mission"]
        self.Timeout: int = 3

    @SaveHtmlToTable
    def MakeGetRequest(self) -> Tuple[List[str], Status, Optional[str]]:

        # This clinic starts as WAITLIST, could change in the future, especially with V2.
        # Currently just set case to WAITLIST and return.

        case = Status.WAITLIST
        return self.Keys, case, None

"""
        Leaving this placeholder code in, as normally (and perhaps in the future
        for this clinic), we would get the HTML, parse it, search through elements
        in the DOM, and maybe change the Status based on FailureCase, etc.
        Right now, this scraper always returns WAITLIST (V1), based on
        information gathered from phone calls to the clinic.

        # Make outbound GET to the URL in question.
        # Specify headers, and a timeout so we catch hangs.

        try:
            r = requests.get(self.URL, headers=self.headers, timeout=self.Timeout)
        except requests.exceptions.RequestException as err:
            logging.debug(f"request exception {err}")
            return self.Keys, case, None

        # Response overloads bool, so r is True for status_codes 200 - 400,
        # otherwise False.
        if not r:
            logging.debug(f"Response Failed with {r.status_code}")
            return self.Keys, case, None

        # Parse the HTML
        soup = BeautifulSoup(r.content, "html.parser")

        # Search through elements in the DOM now
        # listItems = soup.find_all('span')
        listItems = soup.find_all("h4")

        # Everything begins as possible, unless tagged as a NO (V1)
        case = Status.possible

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
"""

if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)

    scraper = UnifyCommunityMission()
    keys, case, text = scraper.MakeGetRequest()

    logging.debug(f"keys={keys} case={case}")
