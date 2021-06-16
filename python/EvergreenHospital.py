import requests
from bs4 import BeautifulSoup
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable
import logging
from typing import Tuple, List
import re

class EvergreenHospital(ScraperBase):


    def __init__(self):
        self.URL = "https://www.evergreenhealth.com/covid-19-vaccine"
        self.RegexFailure = r"\bunable\sto\soffer.*\bfirst\sdose"
        self.LocationName = "EvergreenHospital"
        self.Keys = ["evergreen_hospital"]

    @SaveHtmlToTable
    def MakeGetRequest(self) -> Tuple[List[str], Status, str]:
        r = requests.get(self.URL)
        soup = BeautifulSoup(r.content, 'html.parser')

        # Issue #200: The information moved to a span tag from li
        listItems = soup.find_all('span')

        case = Status.POSSIBLE
        for element in listItems:
            # Issue #200: Use a regex to handle future minor verbiage changes
            if re.search(self.RegexFailure, element.text, re.IGNORECASE):
                print(self.LocationName + " still not scheduling")
                case = Status.NO

        if case == Status.POSSIBLE:
            print(self.LocationName + " site has changed, recheck")

        return self.Keys, case, r.text

if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)
    scraper = EvergreenHospital()
    keys, case, text = scraper.MakeGetRequest()
    logging.debug(f"keys={keys} case={case}")
