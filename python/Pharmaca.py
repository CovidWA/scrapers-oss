from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable
import logging
import requests

class Pharmaca(ScraperBase):
    '''class for a scraper that reports status for a single Pharmaca location'''
    
    def __init__(self, locDict):
        '''initialization method. Takes a locDict dictionary with information
        about a single location. The keys of the location dictionary MUST
        match the self.columns attribute of the PharmacaWrapper class''' 
       
        self.URL = locDict["URL"]
        self.LocationName = locDict["Location"]     # location name
        self.Keys = [locDict["Key"]]                # location key
        self.locDict = locDict                      # save whole location dic

        return

    @SaveHtmlToTable
    def MakeGetRequest(self):
        '''look up availability in the passed dictionary and return status'''
        # Make outbound GET to the URL in question - URL not actually used
       
        case = Status.POSSIBLE
        r = requests.get(self.URL)
        
        # try to look up availability - failure indicates unknown
        try:
            if self.locDict["available"]:
                case = Status.YES
            else:
                case = Status.NO
        except KeyError:
            logging.info("Pharmaca location {} has no availability info".format(locDict["address"])) 
        return self.Keys, case, r.text

