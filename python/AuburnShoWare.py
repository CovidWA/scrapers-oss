import requests
from bs4 import BeautifulSoup
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable
from typing import Tuple, List

class AuburnScraper(ScraperBase):
    
    def __init__(self):
        self.URL = "https://www.kingcounty.gov/depts/health/covid-19/vaccine/distribution.aspx"
        self.FailureCase = "All appointments are currently full, and we are unable to schedule new appointments at this time"
        self.LocationName = "Auburn General Services Administration Complex and Kent Accesso ShoWare Center"
        # New restrictions, removing showare self.Keys = ["Auburn", "kent_accesso_showare_center"]
        self.Keys = ["Auburn"]

    @SaveHtmlToTable
    def MakeGetRequest(self) -> Tuple[List[str], Status, str]:
        r = requests.get(self.URL) 
        soup = BeautifulSoup(r.content, 'html.parser')
        paragraphs = soup.find_all('p')

        case = Status.POSSIBLE
        for element in paragraphs:
            if self.FailureCase in element.text:
                print(self.LocationName + " still not scheduling")
                case = Status.NO
                break

        if case == Status.POSSIBLE:
            print(self.LocationName + " site has changed, recheck")
        
        return self.Keys, case, r.text
