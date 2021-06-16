import requests 
from bs4 import BeautifulSoup
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable



class UWScraper(ScraperBase):

    def __init__(self):
        self.URL = "https://www.uwmedicine.org/coronavirus/vaccine"
        self.FailureCase = "First-dose vaccination appointments are not currently available due to low vaccine supply."
        self.LocationName = "UW"
        self.Keys = ["UWNorthWest", "UWMontlake", "UWValley"]

    @SaveHtmlToTable
    def MakeGetRequest(self):
        r = requests.get(self.URL) 
        soup = BeautifulSoup(r.content, 'html.parser')
        headers = soup.find_all('h2')

        case = Status.POSSIBLE
        for element in headers:
            if element.text == self.FailureCase:
                print(self.LocationName + " still not scheduling")
                case = Status.NO
                break

        if case == Status.POSSIBLE:
            print(self.LocationName + " site has changed, recheck")
        
        return self.Keys, case, r.content
