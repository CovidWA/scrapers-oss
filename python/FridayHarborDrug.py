#!/usr/bin/env python

"""
    Scraper for Friday Harbor Drug. They use a form of Acuity Scheduling, but
    unfortunately they use the 'class' template, whereas the AcuityScheduling.py
    handles sites that use the 'weekly' template. And the class template returns
    all the html in the response packet.

"""
import requests
from bs4 import BeautifulSoup
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable, LIMITED_THRESHOLD
from typing import Tuple, List, Optional
import logging
from time import sleep

class FridayHarborDrug(ScraperBase):

    def __init__(self):
        # The main page url
        self.URL = "https://www.fridayharbordrug.com/vaccination-appointment"
        self.ApiURL = "https://app.acuityscheduling.com/schedule.php?action=showCalendar&fulldate=1&owner=20744225&template=class"

        # Params is the params sent in the requests.POST
        self.Params = "action=showCalendar&fulldate=1&owner=20744225&template=class"

        # DataDict is the data sent in the requets.POST
        self.DataDict = {'type': '', 'calendar': '', 'skip': 'true', 'options[qty]': '1', 'options[numDays]': '5', \
                    'ignoreAppointment': '', 'appointmentType': '', 'calendarID': ''}


        self.LocationName = "Friday Harbor Drug, Friday Harbor, WA"
        self.Keys = ["friday_harbor_drug"]

    @SaveHtmlToTable
    def MakeGetRequest(self) -> Tuple[List[str], Status, Optional[str]]:

        # Start with status UNKNOWN. Status doesn't change to POSSIBLE
        # until the GET succeeds.
        case = Status.UNKNOWN
        try:
            r = requests.get(
                self.ApiURL,
                params=self.Params,
                data=self.DataDict
            )
        except requests.exceptions.RequestException as err:
            logging.debug(f"request exception {err}")
            return self.Keys, case, None

        # Response overloads bool, so r is True for status_codes 200 - 400,
        # otherwise False.
        if not r:
            logging.debug(f"Response Failed with {r.status_code}")
            return self.Keys, case, None

        # Parse the HTML
        soup = BeautifulSoup(r.content, 'html.parser')

        # Everything starts out as POSSIBLE.
        case = Status.POSSIBLE

        """
            Look for the num slots selector. Then loop through getting the number
            of available slots (will be empty if all we get is "spots left" without
            a number). Need to check that the parent is not hidden, otherwise
            we get double the number of open slots.
        """
        slots_available = soup.find_all("div", class_="class-spots num-slots-available-container")

        num_slots = 0
        for slot in slots_available:
            if 'hidden-xs' not in slot.parent.get('class'):
                numStr = slot.span.text
                if numStr != "spots left":
                    num_slots += int(numStr)

        if num_slots:
            case = Status.YES if num_slots > LIMITED_THRESHOLD else Status.LIMITED
        else:
            case = Status.NO

        if case == Status.POSSIBLE:
            # We don't have a NO or a YES. Maybe something changed on the site.
            logging.info(self.LocationName + " site has changed, recheck")

        return self.Keys, case, r.text

if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)

    scraper = FridayHarborDrug()
    keys, case, text = scraper.MakeGetRequest()

    logging.debug(f"keys={keys} case={case}")
