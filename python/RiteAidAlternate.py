# Alternate scraping of https://getmyvaccine.org/zips/99301
import requests 
import json
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable
import logging
from typing import List

class RiteAidAlternate(ScraperBase):

    def __init__(self):
        self.URL = "https://www.riteaid.com/pharmacy/covid-qualifier"
        self.API_URL = "https://vaccine-zip.herokuapp.com/api/zips?zip=99301"
        self.FailureCase = ""
        self.Keys = None  # Set before saving to table
        self.LocationName = "Rite Aid"
        self.store_numbers = []
        for line in open('RiteAidStoreNumbers.csv'):
            self.store_numbers.append(line.strip('\n'))

    def MakeGetRequest(self) -> None:
        """
        This function does NOT save anything to the table. It checks statuses, and calls
        self.SaveToTable multiple times for each key & status. It's a little bit hacky because
        there's no easy way to do multiple keys & statuses yet.
        """
        resp = requests.get(self.API_URL) 
        d = json.loads(resp.text)
        
        available_store_numbers = []
        for location in d['availability']['rite_aid']['data']:
            available_store_numbers.append(location['attributes']['store_number'])

        for store_number in self.store_numbers:
            self.Keys = [f'riteaid_{store_number}']
            status = Status.YES if store_number in available_store_numbers else Status.NO
            self.SaveToTable(status, resp.text)    
            if status == Status.YES:
                print(f'Rite Aid #{store_number} is scheduling')
            else:
                print(f'Rite Aid #{store_number} is not scheduling')

    @SaveHtmlToTable
    def SaveToTable(self, status, html):
        """This function actually saves to the table, is called for each location"""
        return self.Keys, status, html


if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)

    RiteAidAlternate().MakeGetRequest()
