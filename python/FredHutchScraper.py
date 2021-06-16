
import requests 
from bs4 import BeautifulSoup
from ScraperBase import ScraperBase

#DISABLED, do not use  OUTDATED
class FredHutchScraper(ScraperBase):


    def __init__(self):
        self.URL = "https://www.FredHutchhealth.com/covid-19-vaccine"
        self.FailureCase1 = "Book ahead visits are fully booked today."
        self.FailureCase2 = "Book ahead visits are fully booked tomorrow."
        self.LocationName = "FredHutch"

    def basicRequest(self):
        r = requests.get(self.URL)
        soup = BeautifulSoup(r.content, 'html.parser')
        listItems = soup.find_all('li')

        case1 = -1
        case2 = -1
        for element in listItems:
            if self.FailureCase1 in element.text:
                print(self.LocationName + " not scheduling for today")
                case1 = 0
            elif self.FailureCase2 in element.text:
                print(self.LocationName + " not scheduling for tomorrow")
                case2 = 0



        if case1 == -1 or case2 == -1:
            print(self.LocationName + " site has changed, recheck")

        return self.Keys, case
