#!/usr/bin/env python
import requests 
from bs4 import BeautifulSoup
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable
import logging
import re
from typing import Tuple, List

class BainbridgeIslandSeniorCenter(ScraperBase):
    
    def __init__(self):
        self.URL = "https://www.bainbridgewa.gov/1264/COVID-19-Vaccine-Information"

        # The status is just a manually updated description, so attempt to match a few different phrasings
        # that would inidcate there are no more doses
        self.FailureRegex = r"(\W)*(no|out of|(do(\Wnot|n't)\Whave))(\W|\w)*(shots|vaccine|appointments|doses|slots)"
        self.LocationName = "Bainbridge Island Senior Center"
        self.Keys = ["bainbridge_island_sc"]

    @SaveHtmlToTable
    def MakeGetRequest(self) -> Tuple[List[str], Status, str]:
        #Make outbound GET to the URL in question
        r = requests.get(self.URL)

        #Parse the HTML
        soup = BeautifulSoup(r.content, 'html.parser')

        # search through every <strong> tag as these
        # contain header text that indicates there has been an
        # update
        listItems = soup.find_all('strong')

        #Everything begins as possible, unless tagged as a NO
        case = Status.POSSIBLE

        # Every update on the webpage is preceded by a <strong> tag
        # that follows this pattern
        updateHeaderRegex = r"\*\*(.*) Update \*\*"

        # iterate through each strong tag to see which are update headers
        for element in listItems:
            # if we've found an update header, it will be the most recent
            # as updates appear on the page in order of most to least recent
            if re.match(updateHeaderRegex, element.text):
                # we want to check the <p> element directly adjacent to the <strong> tag 
                # that contains the header in order to determine if our failure case is
                # found in the contents of the update
                if re.search(self.FailureRegex, element.findNext('p').text):
                    logging.info(self.LocationName + " still not scheduling")
                    case = Status.NO
                # exit loop after first match -- we've already checked the most recent update
                break

        if case == Status.POSSIBLE:
            #Failure case not met, leave as possible, HTML will be auto uploaded by wrapper function
            logging.info(self.LocationName + " site has changed, recheck")

        return self.Keys, case, r.text

if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)
    scraper = BainbridgeIslandSeniorCenter()
    keys, case, text = scraper.MakeGetRequest()
    logging.debug(f"keys={keys} case={case}")
