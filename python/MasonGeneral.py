import requests 
from bs4 import BeautifulSoup
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable
import logging

class MasonGeneral(ScraperBase):


    def __init__(self):
        self.URL = "https://www.masongeneral.com/about/covid-19"
        self.FailureCase = "All appointments for the vaccine clinic are currently full"
        self.LocationName = ""
        self.Keys = ["mason_g"]

    @SaveHtmlToTable
    def MakeGetRequest(self):
        #Make outbound GET to the URL in question
        r = requests.get(self.URL) 

        #Parse the HTML
        soup = BeautifulSoup(r.content, 'html.parser')

        #Search through elements in the DOM now
        listItems = soup.find_all('li')

        #Everything begins as possible, unless tagged as a NO
        case = Status.POSSIBLE
        for element in listItems:
            if self.FailureCase in element.text:
                #Still a no, flag it
                logging.info(self.LocationName + " still not scheduling")
                case = Status.NO
                return self.Keys, case, r.text

        if case == Status.POSSIBLE:
            #Failure case not met, leave as possible, HTML will be auto uploaded by wrapper function
            logging.info(self.LocationName + " site has changed, recheck")
        
        return self.Keys, case, r.text

if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)
    scraper = MasonGeneral()
    keys, case, text = scraper.MakeGetRequest()
    logging.debug(f"keys={keys} case={case}")