#!/usr/bin/env python

import requests
from bs4 import BeautifulSoup
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable
from SignUpGeniusBaseClass import SignUpGeniusBaseClass
import logging
import csv

class SignUpGeniusScraperWrapper():
    """
    Relies on a .csv file containing three columns in the following order:
    LocationName, URL, and Keys.
    Passes this info to the SignUpGeniusBaseClass constructor.
    """
    def __init__(self):
        self.SignUpGeniusList = 'SignUpGeniusLocations.csv'

    def MakeGetRequest(self):
        """
        Called by ScrapeAllAndSend, which assumes that any object in its
        lstActiveScrapers has a method named MakeGetRequest().
        """
        with open(self.SignUpGeniusList, mode='r') as csv_file:
            csv_reader = csv.reader(csv_file, delimiter=',')

            for row in csv_reader:
            # Try/except to avoid crashing the whole for loop in case there's
            # an error with one of the locations.
                try:
                    print("Processing line " + row[0])
                    LocationName, URL, Keys = row[0], row[1], row[2]
                    scraper = SignUpGeniusBaseClass(URL, LocationName, [Keys])
                    keys, case, text = scraper.MakeGetRequest()
                    print(keys, case)
                except Exception as e:
                    print("Ran into error processing row", row, "in SignUpGeniusScraperWrapper." + str(e))

if __name__ == "__main__":
    scraper_wrapper = SignUpGeniusScraperWrapper()
    scraper_wrapper.MakeGetRequest()
