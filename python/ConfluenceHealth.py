import requests 
from bs4 import BeautifulSoup
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable
from typing import Tuple, List

class ConfluenceHealth(ScraperBase):

    def __init__(self):
        self.URL = "https://www.confluencehealth.org/covid-19-vaccine-information/"
        self.FailureCase = "Due to limited vaccine supply, we do not have enough doses to schedule"
        self.LocationName = "ConfluenceHealth"
        self.Keys = ["confluence_health"]

    @SaveHtmlToTable
    def MakeGetRequest(self) -> Tuple[List[str], Status, str]:
        #Make outbound GET to the URL in question
        r = requests.get(self.URL) 

        #Parse the HTML
        soup = BeautifulSoup(r.content, 'html.parser')

        #Search through elements in the DOM now
        listItems = soup.find_all('b')

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