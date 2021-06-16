#!/usr/bin/env python
import requests 
from bs4 import BeautifulSoup
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable
import logging
from typing import Tuple, List

class EvergreenIntern(ScraperBase):

    def __init__(self):
        self.URL = "https://www.evergreenmd.net/covid-19-updates"
        self.WaitListCase = "For inquiries or COVID-19 vaccine appointment, please email us:"
        self.FailureCase = "the State has not allocated COVID vaccine to our office"
        self.LocationName = "EvergreenInternists"
        self.Keys = ["evergreen_internists"]

    @SaveHtmlToTable
    def MakeGetRequest(self) -> Tuple[List[str], Status, str]:
        #Make outbound GET to the URL in question
        r = requests.get(self.URL)

        #Parse the HTML
        soup = BeautifulSoup(r.content, 'html.parser')

        #Search through elements in the DOM now
        listItems = soup.find_all('p')

        #Everything begins as possible, unless tagged as a NO
        case = Status.POSSIBLE
        for element in listItems:
            if self.FailureCase in element.text:
                logging.info(self.LocationName + " still not scheduling")
                case = Status.NO
                break
            
            if self.WaitListCase in element.text:
                #Still a no, flag it
                logging.info(self.LocationName + " still not scheduling")
                case = Status.WAITLIST

        if case == Status.POSSIBLE:
            #Failure case not met, leave as possible, HTML will be auto uploaded by wrapper function
            logging.info(self.LocationName + " site has changed, recheck")

        return self.Keys, case, r.text

if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)
    scraper = EvergreenIntern()
    keys, case, text = scraper.MakeGetRequest()
    logging.debug(f"keys={keys} case={case}")
