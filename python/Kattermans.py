import requests
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable
from typing import Tuple, List
import logging
from bs4 import BeautifulSoup


class Kattermans(ScraperBase):

    def __init__(self):
        self.URL = "http://kattermans.com/covid-vaccination/schedule-1st-vaccine-appointment/"
        self.FailureCase = "No appointments available at this time"
        self.LocationName = "Kattermans Pharmacy"
        self.Keys = ["kattermans_new"]
        self.headers = {
            'X-WP-Nonce': 'c112528e88',
            'User-Agent': 'Chrome',
        }

    @SaveHtmlToTable
    def MakeGetRequest(self)-> Tuple[List[str], Status, str]:
        r = requests.get(self.URL, headers=self.headers)
        soup = BeautifulSoup(r.content, 'html.parser')

        p = soup.find_all('p')

        case = Status.POSSIBLE
        for element in p:
            if self.FailureCase in str(element):
                case = Status.NO
                break

        if case == Status.POSSIBLE:
            logging.info(self.LocationName + " site has changed, recheck")

        return self.Keys, case, r.text


if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)
    scraper = Kattermans()
    keys, case, text = scraper.MakeGetRequest()
    logging.debug(f"keys={keys} case={case}")
