from enum import Enum
import requests
# import json
import os
import logging
import boto3
from urllib.parse import urlsplit, urlunsplit

# switching from numbers to strings
class Status(Enum):
    NO = 0
    YES = 1
    UNKNOWN = 2
    POSSIBLE = 3
# Note these should never be returned
# APIFAIL maps to Unknown
# Waitlist is manually only
    APIFAILED = 4
    WAITLIST = 5
    CALL = 6
    EMAIL = 7
    WALKIN = 8
    LIMITED = 9

class VaccineType(Enum):
  PFIZER = 'pfizer'
  MODERNA = 'moderna'
  JOHNSON = 'johnson'

LIMITED_THRESHOLD = 5    # number of appointments for limited status

def GetAPIUrl(url):
    if os.environ.get("API_HOSTWA") is not None:
        urlparts = list(urlsplit(url))
        urlparts[1] = os.environ.get("API_HOSTWA")
        return urlunsplit(urlparts)

    return url


def GetSecret():
    if os.environ.get("AWS_EXECUTION_ENV") is not None:
        # AWS Lambda has access to the Parameter Store
        ssm = boto3.client('ssm', 'us-west-2')
        return ssm.get_parameter(Name='secret', WithDecryption=True)['Parameter']['Value']
    # Local development uses secret key
    return os.environ.get('API_SECRETWA')


def GetClinicsData(key_filter: str = None):
    """If key_filter is set, only returns clinics with keys containing key_filter"""

    url = GetAPIUrl('https://api.covidwa.com/v1/get_internal')
    headers = {
        'secret': GetSecret(),
        'Content-Type': 'application/json'
    }
    r = requests.post(url, headers)
    if r.status_code != 200:
        print('Error getting clinics data')
        return []
    data = r.json()['data']

    if key_filter is None:
        return data

    def filter_func(entry):
        if 'key' not in entry:
            return False

        # If key starts with 'X', filter it out. This allows Test to add 'X' to
        # key in airtable to temporarily disable scraping.
        keyX = 'X' + key_filter
        if keyX in entry['key']:
            return False
        else:
            return key_filter in entry['key']

    return filter(filter_func, data)  # Only include entries whose key contains key_filter


def SaveHtmlToTable(func):
    def wrapper(*args):
        print(args[0].Keys)
        scrapeResult = func(*args)

        keys = scrapeResult[0]
        case = scrapeResult[1]
        html = scrapeResult[2]
        if len(scrapeResult) > 3:
            # Get the scraperTags. Convert the set to a list.
            scraperTags = list(scrapeResult[3])
        else:
            scraperTags = []

        URL = GetAPIUrl("https://api.covidwa.com/v1/updater")

        secret = GetSecret()
        if not secret:
            logging.warning("Secret not set, assuming local debug run")
            secret = "test"
            URL = "http://127.0.0.1:3000/v1/updater"
            # Nice Testing Resource: Set URL to http://httpbin.org/post
            # will echo back your entire request
            # URL = "http://httpbin.org/post"

        for uniqueKey in keys:
            payloadDict = {
                'key': uniqueKey,
                'status': case.value,
                'secret': secret,
                'scraperTags': scraperTags,
            }

            files = {'output': ('report.csv', html)}
            response = requests.post(URL, files=files, data=payloadDict)
            if response.status_code != 200:
                logging.error(uniqueKey + " scraping failed to push to table")
            else:
                logging.info(uniqueKey + " updated in table")
        return scrapeResult
    return wrapper

def forcePushKeyResultToDataBase(prepmodResult):
    URL = GetAPIUrl("https://api.covidwa.com/v1/checkandput")

    payloadDict = {}
    payloadDict["key"] = prepmodResult.key

    if prepmodResult.avail_count > LIMITED_THRESHOLD :
        payloadDict["status"] = "Yes"
    elif prepmodResult.avail_count == 0:
        payloadDict["status"] = "No"
    else:
        payloadDict["status"] = "Limited"

    payloadDict["secret"] = GetSecret()
    payloadDict["name"] = prepmodResult.name
    payloadDict["address"] = prepmodResult.address
    payloadDict["url"] = prepmodResult.content_url
    payloadDict["county"] = prepmodResult.county
    payloadDict["city"] = prepmodResult.city

    if prepmodResult.scraperTags:
        payloadDict["scraperTags"] = list(prepmodResult.scraperTags)

    #the api requires html to be sent, even though here it's irrelavent, so empty string will do, kek
    files = {'output': ('report.csv', "")}
    response = requests.post(URL, files=files, data=payloadDict)

    if response.status_code != 200:
        logging.error(prepmodResult.key + " scraping push failed to push to table - /checkandput")
    else:
        logging.info(prepmodResult.key + " updated in table - /checkandput")

#WARNING, DO NOT USE IF YOU DON'T WANT TO FORCE-PUSH (ADD KEY IF NOT EXIST TO MASTER DB)
def ForcePushResultDictToDB(func):
    def wrapper(*args):
        resultDict = func(*args)

        for key, value in resultDict.items():
            forcePushKeyResultToDataBase(value)
        return resultDict
    return wrapper
