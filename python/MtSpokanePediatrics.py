# scraper for Mt.Spokane Pediatrics
#
# The provider uses Signup.com. The scraper is currently specific to Mt.Spokane
# but will be generalized once more sites of this type are available and it's
# clear what is the same/different between sites


import requests
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable
import logging
import json
import datetime

class MtSpokanePediatrics(ScraperBase):
    '''class for a scraper that scrapes Mt. Spokane site'''

    def __init__(self):
        '''initialization'''
        self.Keys = ["signup_mt_spokane_pediatrics"]
        self.URL = "https://signup.com/go/WPTKuKW"

    @SaveHtmlToTable
    def MakeGetRequest(self):
        '''call the API and get the available slots'''

        r = requests.get(self.URL)         # get site HTML - not actually used
        today = datetime.datetime.now().strftime("%Y/%m/%d")
        params = (
            ('accesskey', '6775657374'),
            ('activity_id', '3595405'),
            ('enddate', '2021/12/25'),      # probably won't be need by Christmas
            ('include_comments', 'false'),
            ('include_jobassignments', 'true'),
            ('include_jobs', 'true'),
            ('my_jobs', 'false'),
            ('selected_activity', '3595405'),
            ('startdate', today),
        )

        # call API and convert to json
        response = requests.get('https://signup.com/api/events', params=params)
        respJson = response.json()

        case = Status.NO    # assume no appointments

        # iterate through the dates
        if 'data' in respJson:
            for date in respJson["data"]:
                # iterate through slots on the date
                for slot in date["jobs"]:
                    # check that not all slots are taken
                    if slot["totalassignments"] < slot["quantity"]:
                        case = Status.YES
                        break

        return self.Keys, case, r.text


if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)
    scraper = MtSpokanePediatrics()
    keys, case, text = scraper.MakeGetRequest()
    logging.debug(f"keys={keys} case={case}")
