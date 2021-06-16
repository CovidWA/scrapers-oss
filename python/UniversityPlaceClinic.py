#!/usr/bin/env python

import requests
from bs4 import BeautifulSoup
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable
from typing import Tuple, List
import logging

class UniversityPlaceClinic(ScraperBase):


    def __init__(self):
        self.URL = "https://www.universityplaceclinic.com/"
        self.FailureCaseList = ["we are exclusively offering booster (2nd) doses",
                                "COVID vaccination is no longer offered in our clinic until further notice"]
        self.LocationName = "UniversityPlaceClinic"
        self.Keys = ["university_place_clinic"]

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
            res = [failCase for failCase in self.FailureCaseList if(failCase in element.text)]

            if res:
                #Still a no, flag it
                print(self.LocationName + " still not scheduling")
                case = Status.NO
                break

        if case == Status.POSSIBLE:
            #Failure case not met, leave as possible, HTML will be auto uploaded by wrapper function
            print(self.LocationName + " site has changed, recheck")

        return self.Keys, case, r.text

        #TODO: shows second doses only
        return self.Keys, case, r.text

if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)

    scraper = UniversityPlaceClinic()
    keys, case, text = scraper.MakeGetRequest()

    logging.debug(f"keys={keys} case={case}")
