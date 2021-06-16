import csv
import json
import logging
from datetime import date, timedelta
from typing import Tuple, List
from enum import Enum

import requests

from Common import Status, SaveHtmlToTable
from ScraperBase import ScraperBase


class CostcoWrapper:

    def __init__(self):
        self.urlKeys = {}
        self.urlFrontend = {}

        # get API for each possible vaccine and return if any exist
        # - 119 may be pfizer, 129 moderna, and 137 j&j
        service_types = ["119", "129", "137"]
        service_types_names = {"119": "pfizer", "129": "moderna", "137": "j&j"}

        # obtain all costco url to key mappings
        with open("CostcoScraperLocs.csv") as file:
            reader = csv.reader(file)

            for data in reader:
                # replace month information, labeled START_MONTH and NEXT_MONTH; format must be YYYY-MM-DD
                # follows the timedelta format of the actual API
                url = data[0]

                startMonth = date.today().strftime("%Y-%m-%d")
                nextMonth = (date.today() + timedelta(days=30)).strftime("%Y-%m-%d")

                url = url.replace("START_MONTH", startMonth).replace("NEXT_MONTH", nextMonth)

                for service_id in service_types:
                    url_service_id = url.replace("SERVICE_ID", service_id)
                    self.urlFrontend[url_service_id] = data[1]
                    self.urlKeys[url_service_id] = data[2] + "_" + service_types_names[service_id]

    def MakeGetRequest(self):
        # iterate all costco locations
        frontend_statuses = {}

        for url in self.urlKeys:
            # don't repeatedly check the online status of a particular location for all 3 doses
            frontendStatus = None
            if self.urlFrontend[url] in frontend_statuses:
                frontendStatus = frontend_statuses[self.urlFrontend[url]]

            scraper = Costco(url, self.urlKeys[url], self.urlFrontend[url], frontendStatus)
            keys, case, text = scraper.MakeGetRequest()
            frontend_statuses[self.urlFrontend[url]] = scraper.frontendStatus
            logging.debug(f"Processing Costco for keys={keys}: case={case}")

        return

class CostcoFrontendStatus(Enum):
    ONLINE_NORMAL_SITE = 0
    ONLINE_BOOKNOW_SITE = 1
    OFFLINE = 2

class Costco(ScraperBase):

    def __init__(self, url, key, urlFrontend, frontendStatus = None):
        self.URL = url
        self.Keys = [key]
        self.urlFrontend = urlFrontend
        self.FailureCase = "Something went wrong on our servers while we were processing your request"
        self.frontendStatus = frontendStatus

        return

    def getAppointmentFrontendSiteStatus(self):
        try:
            req = requests.get(self.urlFrontend, timeout=5.0)
        except requests.exceptions.RequestException as err:
            logging.debug(f"request exception {err}")
            return CostcoFrontendStatus.OFFLINE

        if not req:
            return CostcoFrontendStatus.OFFLINE

        if "We’re sorry, but scheduling is not currently available." in req.text:
            return CostcoFrontendStatus.OFFLINE

        if "The site is temporarily disabled. Please check back at a later time." in req.text:
            return CostcoFrontendStatus.OFFLINE

        if "booknow.appointment-plus" in req.url:
            return CostcoFrontendStatus.ONLINE_BOOKNOW_SITE
        else:
            return CostcoFrontendStatus.ONLINE_NORMAL_SITE

    def MakeAlternativeSiteGetRequest(self):
        # currently, unable to find appointment API availability on the new site, so will return UNKNOWN for these
        # locations for now
        return self.Keys, Status.UNKNOWN, None

    @SaveHtmlToTable
    def MakeGetRequest(self) -> Tuple[List[str], Status, str]:
        case = Status.UNKNOWN

        if self.frontendStatus is None:
            self.frontendStatus = self.getAppointmentFrontendSiteStatus()
        if self.frontendStatus == CostcoFrontendStatus.OFFLINE:
            return self.Keys, Status.NO, None
        if self.frontendStatus == CostcoFrontendStatus.ONLINE_BOOKNOW_SITE:
            return self.MakeAlternativeSiteGetRequest()

        # call the API, assuming normal API site operation

        try:
            req = requests.get(self.URL, timeout=5.0)
        except requests.exceptions.RequestException as err:
            logging.debug(f"request exception {err}")
            return self.Keys, case, None

        if not req:
            logging.debug(f"Response Failed with {req.status_code}")
            return self.Keys, case, None

        case = Status.POSSIBLE

        # failure case occurs when costco servers are not working
        if self.FailureCase in req.text:
            return self.Keys, case, None

        data = json.loads(req.text)

        # check for site changes from what is expected
        if len(data) == 0 or "message" not in data or "errors" not in data or "data" not in data or "gridHours" not in \
                data["data"]:
            logging.error("COSTCO", self.URL, "site has changed! recheck")
            return self.Keys, case, None

        # check for appointments
        if len(data["message"]) > 0 and data["message"][0] == "Appointment Grid hours retrieved":
            if data["errors"] is None:
                if type(data["data"]["gridHours"]) == list and len(data["data"]["gridHours"]) == 0:
                    case = Status.NO
                else:
                    case = Status.YES

        return self.Keys, case, req.text


if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)

    scraper = CostcoWrapper()
    scraper.MakeGetRequest()
