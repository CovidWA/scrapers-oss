#!/usr/bin/env python

import requests
from bs4 import BeautifulSoup
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable
import logging
from typing import Tuple, List

class EvergreenState(ScraperBase):

    def __init__(self):
        self.URL = "https://www.signupgenius.com/tabs/13577df01a0cfedc5ac5-vaccine2"
        self.FailureCase = "Already filled"
        self.LocationName = "EvergreenState"
        self.Keys = ["evergreen_state"]

    @SaveHtmlToTable
    def MakeGetRequest(self) -> Tuple[List[str], Status, str]:
        #Make outbound GET to the URL in question
        r = requests.get(self.URL) 

        #Parse the HTML
        soup = BeautifulSoup(r.content, 'html.parser')

        #Search through elements in the DOM now
        listItems = soup.find_all('span', {"class": "SUGsignups"})

        #Everything begins as possible, unless tagged as a NO
        case = Status.NO
        for element in listItems:
            if self.FailureCase not in element.text:
                logging.info(self.LocationName + " site has changed, recheck")
                case = Status.POSSIBLE
                break

        return self.Keys, case, r.text

if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)
    scraper = EvergreenState()
    keys, case, text = scraper.MakeGetRequest()
    logging.debug(f"keys={keys} case={case}")
