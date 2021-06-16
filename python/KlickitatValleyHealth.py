#!/usr/bin/env python

import requests
from bs4 import BeautifulSoup
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable
import logging
from typing import Tuple, List

class KlickitatValleyHealth(ScraperBase):

    def __init__(self):
        self.URL = "http://www.kvhealth.net/index.php/kvh/pages/covid-19-response"
        self.WaitListCase = "To register and make an appointment, call 509.773.4029"
        self.LocationName = "Klickitat Valley Health Department"
        self.Keys = ["klickitat_valley_health"]

    @SaveHtmlToTable
    def MakeGetRequest(self) -> Tuple[List[str], Status, str]:
        # Make outbound GET to the URL in question
        r = requests.get(self.URL)

        # Parse the HTML
        soup = BeautifulSoup(r.content, 'html.parser')

        # Search through elements in the DOM now
        listItems = soup.find_all('h4')

        # Everything begins as possible, unless tagged as a NO
        case = Status.POSSIBLE
        for element in listItems:
            if self.WaitListCase in element.text:
                # This is really the waitlist case, but we still set to
                logging.info(self.LocationName + " must call for appointment")
                
                # case = Status.CALL
                case = Status.WAITLIST

                break

        if case == Status.POSSIBLE:
            # Failure case not met, leave as possible, HTML will be auto uploaded by wrapper function
            logging.info(self.LocationName + " site has changed, recheck")

        return self.Keys, case, r.text


if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)

    scraper = KlickitatValleyHealth()
    keys, case, text = scraper.MakeGetRequest()

    logging.debug(f"keys={keys} case={case}")
