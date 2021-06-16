#!/usr/bin/env python

import requests
from bs4 import BeautifulSoup
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable
import logging
from typing import Tuple, List
from prepmod import prepmod
import re

class EvergreenstatePrepmod(ScraperBase):

    def __init__(self):
        self.URL = "https://snohomish-county-coronavirus-response-snoco-gis.hub.arcgis.com/pages/covid-19-vaccine"
        self.LocationName = "EvergreenState"
        self.Keys = ["evergreen_state_prep"]

    @SaveHtmlToTable
    def MakeGetRequest(self) -> Tuple[List[str], Status, str]:
        #Make outbound GET to the URL in question
        r = requests.get(self.URL)

        #Parse the HTML
        soup = BeautifulSoup(r.content, 'html.parser')

        prepmod_links = []    # list of prepmod links on main website
        #finds monreos prepmod link only
        for place in soup.find_all('ul'):
            if "Monroe" in place.text:
                for link in place.find_all('a', attrs={'href': re.compile("^https://prepmod")}):
                    prepmod_links.append(link.get('href'))

        #create prepmod object
        prep = prepmod(prepmod_links)
        #gets combined status of links
        case = prep.getcase()

        if case == Status.POSSIBLE:
            # Failure case not met, leave as possible
            # HTML will be auto uploaded by wrapper function
            logging.info(self.LocationName + " site has changed, recheck")


        return self.Keys, case, r.text

if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)
    scraper = EvergreenstatePrepmod()
    keys, case, text = scraper.MakeGetRequest()
    logging.debug(f"keys={keys} case={case}")
