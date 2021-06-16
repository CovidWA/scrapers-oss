#!/usr/bin/env python

import requests
from bs4 import BeautifulSoup
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable
import logging
from typing import Tuple, List, Optional
import re

class SignUpGeniusBaseClass(ScraperBase):

    def __init__(self, URL, LocationName, Keys):
        # IMPORTANT: This needs to be the URL to the SignUpGenius webpage
        # that lists all appointments in LIST form, and not in calendar form.
        self.URL = URL
        self.LocationName = LocationName
        self.Keys = Keys
        self.Timeout = 10   # Chosen arbitrarily, but works for the slower sites so far

        # If a slot is booked up, it will show up in a class called
        # "SUGsignups", with the text "Already filled"
        self.FailureCase = "Already filled"

        # Define a SuccessCase. If there are appts available, those will show up
        # in a class called "SUGbutton rounded", with the text "Sign Up"
        # This is based on information from the WallaWallaFairgrounds website,
        # where available appts were observed. Assuming all SignUpGenius
        # websites work that way, WHICH MAY NOT BE TRUE.
        self.SuccessCase = "Sign Up"

    @SaveHtmlToTable
    def MakeGetRequest(self) -> Tuple[List[str], Status, Optional[str]]:

        # Status starts out as Unknown in case we get request exceptions.
        case = Status.UNKNOWN

        # Make outbound GET to the URL in question. Specify timeout.
        try:
            r = requests.get(self.URL, timeout=self.Timeout)
        except requests.exceptions.RequestException as err:
            print(f"GET error {err}")
            logging.debug(f"request exception {err}")
            return self.Keys, case, None

        # Response overloads bool, so r is True for status_codes 200 - 400,
        # otherwise False.
        if not r:
            logging.debug(f"Response Failed with {r.status_code}")
            return self.Keys, case, None

        # Everything begins as possible, now that we made the request successfully.
        case = Status.POSSIBLE

        #Parse the HTML
        soup = BeautifulSoup(r.content, 'html.parser')

        # New behavior observed on 02/23/21. When SignUpGenius pages are full,
        # they will post an h1 banner at the bottom of the page with a message
        # that generally includes 'no slots available', and remove the whole
        # table that used to show the different appt slots
        banners = soup.find_all('h1')

        # Check banners first, because the owner of the page may have posted a banner saying
        # "no slots avaiable". They may or may not have then removed the whole table.
        # So report Status.NO, and don't waste time looking through all the elements in the table,
        # if it still exists.
        # For debugging! Comment out for production
        # print([x.text for x in banners])

        for banner in banners:
            if 'no slots available' in banner.text.lower():
                # print(f"Failure case in banner")
                case = Status.NO

        spans = soup.find_all('span', class_='SUGbigbold')
        for span in spans:
            if re.match(r'(.*)no slots (.*)available', span.text, re.IGNORECASE):
                case = Status.NO

        # Search through elements in the DOM
        # "SUGsignups" is the class where fully booked slots appear
        # "SUGbutton rounder" is the class where available slots appear
        listItems = soup.find_all('span',
                    {"class": ["SUGsignups", "SUGbutton rounded"]}                                )

        if (case == Status.POSSIBLE):

            # Variable unexpected keeps track of whether or not we encounter
            # unexpected values in the HTML (indicates that the webpage changed)
            unexpected = False

            # For debugging! Comment out for production
            # print([x.text for x in listItems])

            for element in listItems:
                # If we find a slot, no need to iterate through the rest of the list
                if self.SuccessCase in element.text:
                    case = Status.YES
                    break

                if self.FailureCase in element.text:
                    # print(f"Failure case in element loop")
                    case = Status.NO

                # If we get to this portion of the code, it means that we have an
                # unexpected string in the DOM. Update variable unexpected to True
                else:
                    logging.info(self.LocationName + " site has changed, recheck")
                    unexpected = True

            # If we haven't found a single available slot, and found some
            # unexpected value in the HTML, flag that and make the case POSSIBLE
            # so we know to check on the scraper.
            # If listItems is empty, means that scraping is not proceeding
            # as usual, so flag that.
            if (case != Status.YES and unexpected == True) or not listItems:
                case = Status.POSSIBLE

        logging.debug(f"Returning Keys = {self.Keys}, case = {case}")

        return self.Keys, case, r.text
