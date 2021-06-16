#!/usr/bin/env python

import requests
from bs4 import BeautifulSoup
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable
import logging
from typing import Tuple, List
import datetime

class KitsapCommunityClinic(ScraperBase):

    def __init__(self):
        # The main page url
        self.URL = "https://kitsappublichealth.org/CommunityHealth/CoronaVirus_Vaccine.php"
        self.alternateURL = "https://kphd.timetap.com/"
        self.LocationName = "Kitsap Public Health District"
        self.Keys = ["kitsap_public_health"]
        
        self.FailureCase = "are now full"    # failure case from website scrape
        self.FailCaseApi = "[]"              # failure from API call
        self.SuccessCase = "openSeats"       # indicates found slot in API

        self.NumDays = 14     # number of dates in the future to check
        self.ApiURL = 'https://kphd.timetap.com/businessWeb/csapi/cs/availability/class/day/<YEAR>/<MONTH>/<DAY>' 
        
        # headers used in request call
        self.headers = {
                'authorization': 'Bearer cst:341260:kphd:ra90a6c1e52d040799ed60909a8a81a9b',
                'content-type': 'application/json',
            }
        # data field used in request call 
        self.data = '{"auditReferralId":null,"debug":false,"locale":"en-US","businessId":341260,"schedulerLinkId":217680,"staffIdList":null,"reasonIdList":[602263],"locationIdList":[467692],"locationGroupIdList":null,"reasonGroupIdList":null,"locationSuperGroupIdList":null,"reasonSuperGroupIdList":null,"classScheduleIdList":null,"groupIdList":null,"clientTimeZone":"America/Los_Angeles","clientTimeZoneId":66,"filterLocation":null,"address":"2520 Cherry Avenue Bremerton, WA 98310","businessTimeZone":"America/Los_Angeles","businessTimeZoneId":66}'
        

    @SaveHtmlToTable
    def MakeGetRequest(self):
        # Make outbound GET to the URL in question
        r = requests.get(self.alternateURL)

        # Parse the HTML
        soup = BeautifulSoup(r.content, 'html.parser')

        # Search through elements in the DOM now
        listItems = soup.find_all('div')

        # Everything begins as possible, unless tagged as a NO
        case = Status.POSSIBLE

        # try to scrape website for failure case first
        for element in listItems:
            if self.FailureCase in element.text:
                logging.info(self.LocationName + " is full")

                case = Status.NO
                break

        # did not fail - try to confirm availability using API call
        if case == Status.POSSIBLE:
            
            failDates = 0           # count days w/o appointments
            for i in range(self.NumDays):
                # construct the URL for API call - update date 
                date = datetime.date.today() + datetime.timedelta(days=i)
                URL = self.ApiURL.replace("<YEAR>", str(date.year))
                URL = URL.replace("<MONTH>", str(date.month))
                URL = URL.replace("<DAY>", str(date.day))
                
                # attempt to make request
                try:
                    response = requests.post(
                         URL, 
                         headers=self.headers, 
                         data=self.data
                    )
                except:
                    continue    # if request failed move on to next day
                
                # found a date with appointment
                if self.SuccessCase in response.text:
                    case = Status.YES
                    break

                # confirmed no appointment
                if self.FailCaseApi == response.text:
                    failDates += 1

        # every scanned date had no appointment - confirmed no
        if failDates == self.NumDays:
            case = Status.NO

        if case == Status.POSSIBLE:
            # Failure case not met, leave as possible, HTML will be auto uploaded by wrapper function
            logging.info(self.LocationName + " site has changed, recheck")

        return self.Keys, case, r.text


if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)
    scraper = KitsapCommunityClinic()
    keys, case, text = scraper.MakeGetRequest()
    logging.debug(f"keys={keys} case={case}")
