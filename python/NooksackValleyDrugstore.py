#!/usr/bin/env python
import requests 
from bs4 import BeautifulSoup
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable
import logging
import re
from typing import Tuple, List

class NooksackValleyDrugstore(ScraperBase):

    def __init__(self):
        self.URL = "https://nooksackvalleydrug.com"
        self.FailureCase = "Currently, we have no COVID-19 vaccine available."
        self.LocationName = "NooksackValleyDrugstore"
        self.Keys = ["nooksack_everson"]

    @SaveHtmlToTable
    def MakeGetRequest(self) -> Tuple[List[str], Status, str]:
        #Make outbound GET to the URL in question
        r = requests.get(self.URL) 

        #Parse the HTML
        soup = BeautifulSoup(r.content, 'html.parser')
        paragraphs = soup.find_all('b')

        case = Status.POSSIBLE
        for element in paragraphs:
            if self.FailureCase in element.text:
                logging.info(self.LocationName + " still not scheduling")
                case = Status.NO
                break

        if case == Status.POSSIBLE:
            logging.info(self.LocationName + " site has changed, recheck")
        
        return self.Keys, case, r.text

if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)
    scraper = NooksackValleyDrugstore()
    keys, case, text = scraper.MakeGetRequest()
    logging.debug(f"keys={keys} case={case}")
    logging.basicConfig
