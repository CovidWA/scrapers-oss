import requests 
from bs4 import BeautifulSoup
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable
import logging

class LakeChelan(ScraperBase):


    def __init__(self):
        self.URL = "https://lakechelanhealth.org/covid-19/"
        self.FailureCase = "did not receive vaccine"
        self.LocationName = ""
        self.Keys = ["lk_chelan"]

    @SaveHtmlToTable
    def MakeGetRequest(self):
        #Make outbound GET to the URL in question
        headers = {"User-Agent":"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_4) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.97 Safari/537.36"}

        
        r = requests.get(self.URL, headers = headers) 

        
        #Parse the HTML
        soup = BeautifulSoup(r.content, 'html.parser')

        #Search through elements in the DOM now
        listItems = soup.select('p')
        
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
    scraper = LakeChelan()
    keys, case, text = scraper.MakeGetRequest()
    logging.debug(f"keys={keys} case={case}")