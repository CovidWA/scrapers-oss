import requests 
from bs4 import BeautifulSoup
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable
import logging
import re
from typing import Tuple, List


class CamanoIslandHealthSystem(ScraperBase):

    def __init__(self):
        self.URL = "https://www.islandcountywa.gov/Health/Pages/Covid-Vaccine.aspx"
        self.FailureCase = "For Camano Island Health System's Clinic, the appointments are full, currently for second doses."
        self.LocationName = "CamanoIslandHealthSystem"
        self.Keys = ["camano_island_hs"]

    @SaveHtmlToTable
    def MakeGetRequest(self) -> Tuple[List[str], Status, str]:
        #Make outbound GET to the URL in question
        r = requests.get(self.URL) 

        #Parse the HTML
        soup = BeautifulSoup(r.content, 'html.parser')
        paragraphs = soup.find_all('span')

        case = Status.POSSIBLE
        for element in paragraphs:
            if self.FailureCase in element.text:
                print(self.LocationName + " still not scheduling")
                case = Status.NO
                break

        if case == Status.POSSIBLE:
            print(self.LocationName + " site has changed, recheck")
        
        return self.Keys, case, r.text

if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)
    scraper = CamanoIslandHealthSystem()
    keys, case, text = scraper.MakeGetRequest()
    logging.debug(f"keys={keys} case={case}")
    logging.basicConfig
