#!/usr/bin/env python

import logging
import datetime
import json
from typing import Tuple, List, Optional

import requests
from bs4 import BeautifulSoup

from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable

"""
    Cascade Health Clinic is currently using a square booking website to manage appointments.
    API is simple public POST with JSON request/reply. The scraper currently decides yes/no
    based on the length of the 'availabilty' array.
"""

class CascadeHealthClinic(ScraperBase):
    def __init__(self):
        self.URL = "https://squareup.com/appointments/api/buyer/availability"
        self.Keys = [ "cascade_health_clinic" ]
        self.Headers = {
            'content-type': 'application/json; charset=UTF-8',
            'origin': 'https://squareup.com' }

    # NOTE: API only supports a query window of 31 days
    def _query_range(self, start, end):
        query = """
  {"search_availability_request":{"query":{"filter":{
        "start_at_range":{"start_at":"__START__T00:00:00-08:00","end_at":"__END__T00:00:00-08:00"},
        "location_id":"APKNPWJJ9FM2H","segment_filters":[{"service_variation_id":"BR7YWSMDMZQFKSD6C6CC5CRU","team_member_id_filter":{"any":["TMdmEEFi8DJ7dAPG","TMdCg2IvxjEtNDUR"]} }]
  }}}} """
        query = query.replace('__START__', start.isoformat())
        query = query.replace('__END__', end.isoformat())
        return query

    @SaveHtmlToTable
    def MakeGetRequest(self) -> Tuple[List[str], Status, Optional[str]]:
        # Everything begins as UNKNOWN until the POST request succeeds
        case = Status.UNKNOWN

        start = datetime.date.today()
        end = start + datetime.timedelta(days=30)
        query = self._query_range(start, end)

        try:
            resp = requests.post(self.URL, headers=self.Headers, data=query)
        except requests.exceptions.RequestException as err:
            logging.debug(f"request exception {err}")
            return self.Keys, case, None

        # Now that the POST succeeded, case changes to POSSIBLE
        case = Status.POSSIBLE
        try:
            case = Status.YES if len(resp.json()['availability']) else Status.NO
        except ( KeyError, ValueError ):
            logging.error("unexpected query reply")

        return self.Keys, case, resp.text

if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)
    scraper = CascadeHealthClinic()
    keys, case, text = scraper.MakeGetRequest()
    logging.debug(f"keys={keys} case={case} text={text}")
