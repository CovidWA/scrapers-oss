import re
import requests
from ScraperBase import ScraperBase
from Common import GetClinicsData, Status, SaveHtmlToTable, LIMITED_THRESHOLD
import logging
import json

class HealthMartPharmacies(ScraperBase):

    def __init__(self):
        self.URL = "https://healthmartcovidvaccine.com"
        # API URL:  https://scrcxp.pdhi.com/ScreeningEvent/fed87cd2-f120-48cc-b098-d72668838d8b/GetLocations/98072?state=WA
        self.ApiUrl = "https://scrcxp.pdhi.com/Screenings/INSERT_LOCATION_ID/VisibleTimeSlots"
        self.LocationName = "Health Mart Pharmacies"
        self.clinics = GetClinicsData(key_filter='health_mart')

    @staticmethod
    def NormalizeAddress(address):
        """Removes punctuation, whitespace, and capitalization that might make addresses unequal"""
        address = address.lower()
        characters_to_remove = '-,. '
        return re.sub(f'[{characters_to_remove}]', '', address)

    def MakeGetRequest(self):
        results = []

        for table_row in self.clinics:
            # Make outbound GET to the API URL for the zip code of the location in question
            # resp = requests.get(table_row['alternateUrl'], verify = False)

            # API call now requires userAge: e.g., '&userAge=42' fixes things.
            # If alternateUrl doesn't include that, then add it in here. Assumes
            # that if the url doesn't end in WA, then the correct userAge stuff
            # was already appended in airtable.
            url = table_row['alternateUrl']
            if url.endswith("WA"):
                url += '&userAge=42'

            resp = requests.get(url, verify = False)

            # resp = requests.get(table_row['alternateUrl'])

            respJson = resp.json()

            status = Status.NO
            # Loop through returned locations that all have availability
            for location in respJson:
                # Make sure a returned location matches the table_row we are currently checking
                table_address = self.NormalizeAddress(table_row['scraper_config'])  # Street address
                location_address = self.NormalizeAddress(location['address1'])

                if location_address in table_address:
                    # Pick off the locationId to use in the subsequent API call.
                    locationId = location['locationId']

                    # Call the API endppoint to get the visibleTimeSlots list.
                    apiResp = requests.get(self.ApiUrl.replace('INSERT_LOCATION_ID', str(locationId)), verify = False)
                    apiJson = apiResp.json()
                    numVisibleTimeSlots = len(apiJson['visibleTimeSlots'])
                    # print(f"loc: {location_address}, locationId: {locationId}, #slots: {numVisibleTimeSlots}")

                    if numVisibleTimeSlots > LIMITED_THRESHOLD:
                        status = Status.YES
                    elif numVisibleTimeSlots > 0:
                        status =  Status.LIMITED
                    break

            self.Keys = [table_row['key']]
            self.SaveToTable(status, resp.text)

            results.append(f'{self.Keys[0]} : {status.name}')

        return results  # Used for single file testing

    @SaveHtmlToTable
    def SaveToTable(self, status, html):
        return self.Keys, status, html

if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)

    scraper = HealthMartPharmacies()
    results = scraper.MakeGetRequest()

    for result in results:
        logging.debug(result)
