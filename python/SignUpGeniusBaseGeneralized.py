#!/usr/bin/env python

import requests
from bs4 import BeautifulSoup
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable, LIMITED_THRESHOLD
import logging
from typing import Tuple, List, Optional
import re

class SignUpGeniusBaseGeneralized(ScraperBase):

    def __init__(self, URLs, LocationName, Keys):
        # IMPORTANT: This needs to be the list of URLs to the SignUpGenius webpages
        # that list all appointments in LIST form, and not in calendar form.
        self.URLs = URLs
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

        # Make sure we have at least one URL to scrape.
        if not self.URLs:
            logging.info(f"URL list is empty, returning NO")
            return self.Keys, Status.NO, None

        statusDict = {'NO':0, 'YES':0, 'UNKNOWN':0, 'POSSIBLE':0, 'LIMITED':0}

        # Check all the urls in the list.
        for url in self.URLs:

            # Make outbound GET to the URL in question. Specify timeout.
            try:
                r = requests.get(url, timeout=self.Timeout)
            except requests.exceptions.RequestException as err:
                # Log the error and continue the for loop, skipping this url
                print(f"GET error {err}")
                logging.debug(f"request exception {err}")
                statusDict['UNKNOWN'] += 1
                continue

            # Response overloads bool, so r is True for status_codes 200 - 400,
            # otherwise False.
            if not r:
                logging.debug(f"Response Failed with {r.status_code}")
                statusDict['UNKNOWN'] += 1
                continue

            # Parse the HTML
            soup = BeautifulSoup(r.content, 'html.parser')

            # New behavior observed on 02/23/21. When SignUpGenius pages are full,
            # they might post an h1 banner at the bottom of the page with a message
            # that generally includes 'no slots available', and remove the whole
            # table that used to show the different appt slots
            h1Tags = soup.find_all('h1')

            # Check banners and other <h1> first, because the owner of the page may have posted a banner saying
            # "no slots avaiable", etc. They may or may not have then removed the whole table.
            # So report Status.NO, and don't waste time looking through all the elements in the table,
            # if it still exists.

            for h1Tag in h1Tags:
                re_list = ['(.*)no slots (.*)available', '(.*)sign up (.*)not found']
                generic_re = re.compile('|'.join(re_list), re.IGNORECASE)
                if re.match(generic_re, h1Tag.text.lower()):
                    # print(f"Failure case in banner/h1")
                    statusDict['NO'] += 1
                    continue

            spans = soup.find_all('span', class_='SUGbigbold')
            for span in spans:
                if re.match(r'(.*)no slots (.*)available', span.text, re.IGNORECASE):
                    statusDict['NO'] += 1
                    continue

            # Look for div with class_= redmessage, for
            # 'sign up period for this event has ended'.

            redmessages = soup.find_all('div', class_='redmessage')
            for redmsg in redmessages:
                if re.match(r'(.*)sign up period (.*) has ended', redmsg.text.strip(), re.IGNORECASE):
                    statusDict['NO'] += 1
                    continue

            # Search through elements in the DOM.
            # "SUGsignups" is the class where fully booked slots appear
            # "SUGbutton rounder" is the class where available slots appear
            listItems = soup.find_all('span',
                        {"class": ["SUGsignups", "SUGbutton rounded"]}                                )

            # Variable unexpected keeps track of whether or not we encounter
            # unexpected values in the HTML (indicates that the webpage changed).
            unexpected = False

            for element in listItems:

                if self.SuccessCase in element.text:
                    statusDict['YES'] += 1
                    # break

                elif self.FailureCase in element.text:
                    # print(f"Failure case in element loop")
                    statusDict['NO'] += 1

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
            if (statusDict['YES'] == 0 and unexpected == True) or not listItems:
                statusDict['POSSIBLE'] += 1

        # print(f"statusDict: {statusDict}")

        if statusDict['YES'] > LIMITED_THRESHOLD:
            # I wouldn't call the YES count accurate as far as actual number of
            # APPOINTMENTS available; rather, it is the number of TIME SLOTS that
            # still have at least 1 availability. There might be more than 1
            # availability in that time slot. But this is good enough to determine
            # LIMITED.
            case = Status.YES
        elif statusDict['LIMITED']:
            case = Status.LIMITED
        elif statusDict['NO']:
            case = Status.NO
        elif statusDict['POSSIBLE']:
            case = Status.POSSIBLE
        elif statusDict['UNKNOWN']:
            case = Status.UNKNOWN

        # logging.debug(f"Returning Keys = {self.Keys}, case = {case}")

        if case != Status.UNKNOWN:
            return self.Keys, case, r.text
        else:
            return self.Keys, case, None
