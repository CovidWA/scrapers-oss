import requests 
from bs4 import BeautifulSoup
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable
from typing import Tuple, List

class FerryCountyHealth(ScraperBase):

    def __init__(self):
        self.URL = "https://ferrycountyhealth.com/"
        self.FailureCase = "the Washington State Department of Health has denied our vaccine request and will NOT be sending us any primary (first) doses"
        self.LocationName = "FerryCountyHealth"
        self.Keys = ["ferry_county_health"]

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
                #Still a no, flag it
                print(self.LocationName + " still not scheduling")
                case = Status.NO
                break

        if case == Status.POSSIBLE:
            #Failure case not met, leave as possible, HTML will be auto uploaded by wrapper function
            print(self.LocationName + " site has changed, recheck")
        
        return self.Keys, case, r.text