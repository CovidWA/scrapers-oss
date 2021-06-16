from datetime import datetime
import requests 
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable

class WalgreensScraper(ScraperBase):
    def __init__(self, latitude:float, longitude:float, locationName:str, keyNames:[str]):
        self.URL = "https://www.walgreens.com/hcschedulersvc/svc/v1/immunizationLocations/availability"
        self.LocationName = locationName
        self.Keys = keyNames
        self.Latitude = latitude
        self.Longitude = longitude
        self.FailureCase = "\"appointmentsAvailable\":false"

    @SaveHtmlToTable
    def MakeGetRequest(self):
        #Make outbound POST to the URL in question
        #ServiceID and radius have to stay static from what I've seen. radius is an int, service Id a string.
        print("Querying Walgreens for location " + self.LocationName)
        r = requests.post(self.URL, 
            json = {'appointmentAvailability': {'startDateTime' : datetime.now().strftime("%Y-%m-%d")}, 'position': {'latitude': self.Latitude, 'longitude':self.Longitude}, 'radius' : 25, 'serviceId':"99" }) 
        print("Request returned with http " + str(r.status_code))
        case = Status.UNKNOWN
        if(r.status_code < 300):
            responseJson = r.json()
            if(responseJson['appointmentsAvailable'] == True):
                case = Status.YES
            else:
                case = Status.NO

        return self.Keys, case, r.text