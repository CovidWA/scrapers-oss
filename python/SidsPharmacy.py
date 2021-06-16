#!/usr/bin/env python

import requests
from bs4 import BeautifulSoup
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable
import logging


class SidsPharmacy(ScraperBase):

    def __init__(self):
        # The main page url
        self.URL = "http://sidspharmacy.com"
        self.WaitListCase = "add your name to the wait list, please call"
        self.LocationName = "Sid's Pharmacy, Pullman, WA"
        self.Keys = ["sids_pharmacy_pullman"]

    @SaveHtmlToTable
    def MakeGetRequest(self):
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
                # POSSIBLE should really be WAITLIST or CALL
                logging.info(self.LocationName + " call to add to waitlist")

                case = Status.WAITLIST

                break

        if case == Status.POSSIBLE:
            # Failure case not met, leave as possible, HTML will be auto uploaded by wrapper function
            logging.info(self.LocationName + " site has changed, recheck")

        return self.Keys, case, r.text


if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)

    scraper = SidsPharmacy()
    keys, case, text = scraper.MakeGetRequest()

    logging.debug(f"keys={keys} case={case}")
