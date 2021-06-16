import requests
from bs4 import BeautifulSoup
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable
from typing import Tuple, List
from prepmod import prepmod
import logging
import re

class SanJuan(ScraperBase):

    def __init__(self):
        self.URL = "https://www.sanjuanco.com/1737/COVID-Vaccine-Info"
        self.LocationName = "San Juan Island"
        self.Keys = {"SJHCS_SanJuan_Island"}

    @SaveHtmlToTable
    def MakeGetRequest(self)-> Tuple[List[str], Status, str]:
        #Make outbound GET to the URL in question
        r = requests.get(self.URL)

        #Parse the HTML
        soup = BeautifulSoup(r.content, 'html.parser')

        prepmod_links = []    # list of prepmod links on main website
        for element in soup.find_all(['a', 'span'], href=re.compile("^https://")):
            if element['href'].startswith("https://prepmod"):
                location = element.find_previous('a')
                if location is None:
                    location = element.find_previous('span').find_previous('span').text.lower()
                else:
                    location = location.text.lower()
                if "san juan county fairgrounds" in location:
                    prepmod_links.append(element['href'])

        # print(f"prepmod_links: {prepmod_links}")

        links = prepmod(prepmod_links)
        case = links.getcase()

        if case == Status.POSSIBLE:
            # Failure case not met, leave as possible
            # HTML will be auto uploaded by wrapper function
            logging.info(self.LocationName + " site has changed, recheck")

        return self.Keys, case, r.text

if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)
    scraper = SanJuan()
    keys, case, text = scraper.MakeGetRequest()
    logging.debug(f"keys={keys} case={case}")
    logging.basicConfig
