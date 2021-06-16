"""
This scraper is for the Island Drug sites using PrepMod direct link scheduling.
Each location, url, value to match in the href, and  key is stored in
IslandDrugLocations.csv. The IslandDrugWrapper loads the data from
the csv file, and the MakeAllRequests method creates an IslandDrugPrepMod
class for each location. IslandDrugPrepMod makes use of the imported prepmod to
process the prepmod link.
"""

import requests
from bs4 import BeautifulSoup
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable
import logging
import csv
from typing import Tuple, List, Optional
from prepmod import prepmod
import re


class IslandDrugWrapper():
    '''
        Wrapper function for IslandDrugPrepMod locations. Instantiates an
        instance of the IslandDrugPrepMod class for each location.
    '''
    def __init__(self):
        '''
            Initialize the IslandDrugWrapper class and call a method
            to load location data from the csv file.
        '''

        self.Locations = []        # list of dictionaries holding loc info

        # Columns from IslandDrugLocations.csv - these should match the CSV.
        # N.B. The Href_Value should be lowercase in the CSV, as I'm doing
        # lowercase matching.
        self.columns = ["Location", "URL", "Href_Value", "Key", "FailureCase"]
        self.ReadLocations()

        # DEBUGGING ONLY
        # print(f"self.Locations: {self.Locations}")     # load the file of locations

    def ReadLocations(self):
        '''
            Open the csv storing location data and read into self.Locations
        '''

        with open("IslandDrugLocations.csv") as csvfile:
            locReader = csv.reader(csvfile)

            for row in locReader:
                locationDic = {}   # dictionary holding data for each location

                # first row has the column headers - skip this row
                if row[0] == self.columns[0]:
                    continue

                # add columns to dictionary
                for i, col in enumerate(self.columns):
                    locationDic[col] = row[i]

                # add locations dict to the list
                self.Locations.append(locationDic)

        return

    def MakeGetRequest(self):
        '''
            Create an IslandDrugPrepMod scraper class for each location and
            call the scraper's MakeGetRequest method to scrape the location
            and update the airtable.
        '''

        cols = self.columns

        # iterate through all the locations
        for loc in self.Locations:
            scraper = IslandDrugPrepMod(loc)
            keys, case, text = scraper.MakeGetRequest()
            logging.debug(f"Processing IslandDrug for keys={keys}: case={case}")

        return


class IslandDrugPrepMod(ScraperBase):
    '''
        Class for a scraper that looks at a single location in the Island Drug sites.
    '''

    def __init__(self, locDict):
        '''
            Initialization method. Takes a locDict dictionary with information
            about a single location. The keys of the location dictionary MUST
            match the self.columns attribute of the IslandDrugWrapper
        '''

        self.LocationName = locDict["Location"]     # location name
        self.URL = locDict["URL"]                   # URL for the get request
        self.Href_Value = locDict["Href_Value"]     # value to match in the href
        self.Keys = [locDict["Key"]]                # location key
        self.FailureCase = locDict["FailureCase"]   # failure case depicting unavailable

        return

    @SaveHtmlToTable
    def MakeGetRequest(self)-> Tuple[List[str], Status, str]:

        # Starts out as UNKNOWN until the GET request succeeds
        case = Status.UNKNOWN

        # Make outbound GET to the URL in question
        try:
            r = requests.get(self.URL)

        except requests.exceptions.RequestException as err:
            logging.debug(f"request exception {err}")
            return self.Keys, case, None

        # Response overloads bool, so r is True for status_codes 200 - 400,
        # otherwise False.
        if not r:
            logging.debug(f"Response Failed with {r.status_code}")
            return self.Keys, case, None

        # Now everything starts as POSSIBLE.
        case = Status.POSSIBLE

        soup = BeautifulSoup(r.content, 'html.parser')

        prepmod_links = []

        # N.B. This href is 'http' rather than 'https'
        for link in soup.find_all('a', attrs={'href': re.compile("^https?://prepmod")}):
            if self.Href_Value in link.text.lower():
                prepmod_links.append(link.get('href'))

        """
            Let the prepmod scraper process the prepmod links.
        """
        links = prepmod(prepmod_links, self.FailureCase)
        case = links.getcase()

        if case == Status.POSSIBLE:
            # Failure case not met, leave as Possible.
            # HTML will be auto uploaded by wrapper function
            logging.info(self.LocationName + " site has changed, recheck")

        return self.Keys, case, r.text


if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)

    scraper = IslandDrugWrapper()
    scraper.MakeGetRequest()
