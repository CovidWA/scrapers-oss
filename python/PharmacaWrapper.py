import logging
import csv
from datetime import datetime
import requests
import json
from Pharmaca import Pharmaca

class PharmacaWrapper():
    '''wrapper function for Pharmaca locations. Instantiates an
    instance of the Pharmaca class for each location'''
    def __init__(self): 
        '''initialize the Pharmaca class and call a method 
        to load the csv with different location data'''

        self.CsvLocations = []   # list of dictionaries holding loc info from csv
        self.ApiLocations = []   # locations from vaccine-finder site

        # columns from PharmacaLoc.csv - these should match the CSV
        self.columns = ["Address", "Key", "URL", "Location"] 
        self.ApiURL = 'https://www.vaccinespotter.org/api/v0/stores/WA/pharmaca.json'
        self.csvFile = "PharmacaLoc.csv"
        self.headers = {
            'if-modified-since': "<DATE_STRING>"
        }

        self.ReadLocations()       # load the file of locations

    def ReadLocations(self):
        '''open the csv storing location data and read into self.CsvLocations.
        Also call the API to read availability information from 
        vaccine-finder site. Combine these two dictionaries to get full info
        on each site'''
 
        # read information from vaccine-finder API
        now = datetime.utcnow()     # construct time string
        timeString = now.strftime("%a, %d %B %Y %H:%M:%S GMT")

        self.headers['if-modified-since'] = timeString

        r = requests.get(self.ApiURL, headers=self.headers)
        self.ApiLocations = r.json()

        # read location information from CSV
        with open(self.csvFile) as csvfile:
            locReader = csv.reader(csvfile)

            for row in locReader:
                locationDic = {}   # dictionary holding data for each location
                
                # first row has the column headers - skip this row 
                if row[0] == self.columns[0]:
                    continue
                
                # add columns to dictionary 
                for i, col in enumerate(self.columns):
                    locationDic[col] = row[i]
                # add locations dic to the list 
                self.CsvLocations.append(locationDic)


        # combine the Api availability data w/ the CSV location info
        for ApiLoc in self.ApiLocations:
            match = False             # whether Api location has match
            for CsvLoc in self.CsvLocations:
                if ApiLoc["address"] == CsvLoc["Address"]:
                    CsvLoc["available"] = ApiLoc["appointments_available"]
                    match = True
            
            # failed to match - may indicate new location
            if not match:
                logging.info("Pharmaca location {} has no match in CSV".format(ApiLoc["address"]))

        return

    def MakeGetRequest(self):
        '''create a Pharmaca scraper class for each location and
        call method to scrape location and update air table'''
        
        cols = self.columns
        
        # iterate through all the locations
        for loc in self.CsvLocations:
            scraper = Pharmaca(loc)
            keys, case, text = scraper.MakeGetRequest()
            logging.debug(f"keys={keys} case={case}")

        return

if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)
    scraper = PharmacaWrapper()
    scraper.MakeGetRequest()
