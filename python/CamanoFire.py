import requests 
from bs4 import BeautifulSoup
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable
from typing import Tuple, List
from prepmod import prepmod
import logging
import re

class CamanoFire(ScraperBase):

    def __init__(self):
        self.URL = "https://camanofire.com/resources-public-education/covid-19-vaccines/"
        self.FailureCase = "Deadline to register for this clinic has been reached. Please check other clinics."
        self.SuccessCase = "At this moment, these appointments are available" 
        self.LocationName = "CamanoFire"
        self.Keys = ["camano_fire"]

    @SaveHtmlToTable
    def MakeGetRequest(self)-> Tuple[List[str], Status, str]:
        #Make outbound GET to the URL in question
        r = requests.get(self.URL) 

        #Parse the HTML
        soup = BeautifulSoup(r.content, 'html.parser')

        prepmod_links = []    # list of prepmod links on main website
        for link in soup.find_all('a', attrs={'href': re.compile("^https://prepmod")}):
            prepmod_links.append(link.get('href'))

        links = prepmod(prepmod_links)
        case = links.getcase()

        if case == Status.POSSIBLE:
            # Failure case not met, leave as possible
            # HTML will be auto uploaded by wrapper function
            logging.info(self.LocationName + " site has changed, recheck")

        return self.Keys, case, r.text

if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)
    scraper = CamanoFire()
    keys, case, text = scraper.MakeGetRequest()
    logging.debug(f"keys={keys} case={case}")
    logging.basicConfig
