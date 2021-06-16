#!/usr/bin/env python

import requests
from bs4 import BeautifulSoup
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable, GetClinicsData
from SignUpGeniusBaseGeneralized import SignUpGeniusBaseGeneralized
import logging
import csv

class SignUpGeniusWrapperGeneralized():
    """
    Get clinic data from airtable. Pass Name, list of ApiURLs, and Keys to
    the SignUpGeniusBaseGeneralized constructor.
    """
    def __init__(self):
        self.LocationName = ''
        self.Clinics = GetClinicsData(key_filter="signupgenius")

    def GetTabUrls(self, altUrl):
        returnUrls = []

        try:
            r = requests.get(altUrl)
        except requests.exceptions.RequestException as err:
            print(f"GET error {err}")
            logging.debug(f"request exception {err}")
            return returnUrls

        # Response overloads bool, so r is True for status_codes 200 - 400,
        # otherwise False.
        if not r:
            logging.debug(f"Response Failed with {r.status_code}")
            return returnUrls

        # print(f"In GetTabUrls checking for links here: {altUrl}")

        # Parse the HTML looking for tabItems
        soup = BeautifulSoup(r.content, 'html.parser')

        tabItems = soup.find_all('a', class_='tabItem')

        for tab in tabItems:
            onclick = tab.get('onclick')
            link = onclick.replace("javascript:checkFormChanges('", 'https://www.signupgenius.com/go/').replace("')",'')
            returnUrls.append(link)

        return returnUrls

    def GetStaticUrls(self, url):
        returnUrls = []

        try:
            r = requests.get(url)
        except requests.exceptions.RequestException as err:
            print(f"GET error {err}")
            logging.debug(f"request exception {err}")
            return returnUrls

        # Response overloads bool, so r is True for status_codes 200 - 400,
        # otherwise False.
        if not r:
            logging.debug(f"Response Failed with {r.status_code}")
            return returnUrls

        # print(f"In GetStaticUrls checking for links here: {url}")

        # Parse the HTML looking for signupgenius urls
        soup = BeautifulSoup(r.content, 'html.parser')

        links = soup.select('a[href^="https://www.signupgenius.com/go/"]')

        for link in links:
            returnUrls.append(link['href'])

        return returnUrls

    def MakeGetRequest(self):
        """
        Called by ScrapeAllAndSend, which assumes that any object in its
        lstActiveScrapers has a method named MakeGetRequest().
        """
        for row in self.Clinics:
            # Try/except to avoid crashing the whole for loop in case there's
            # an error with one of the locations.
            # print(f"row: {row}")
            try:
                LocationName = row['name']
                # If URL is empty, give up and continue on to the next clinic.
                if 'url' not in row:
                    logging.error(f"Cannot scrape an empty URL! Skipping {LocationName}")
                    continue

                URL, Keys = row['url'], row['key']

                ApiURLs = []
                # print(f"Processing line: {LocationName}")

                if 'alternateUrl' in row:
                    if 'signupgenius.com' in row['alternateUrl']:
                        # If /go/, then do a direct scrape of that one url.
                        if '/go/' in row['alternateUrl']:
                            # Direct scrape of one link.
                            ApiURLs.append(row['alternateUrl'])
                        elif '/tabs/' in row['alternateUrl']:
                            # Direct scrape of potentially multiple tabs, create
                            # a list of urls to scrape.
                            # print(f"Get links from tabs")
                            ApiURLs.extend(self.GetTabUrls(row['alternateUrl']))
                        else:
                            # Just in case the static page to be scraped for ApiURLs
                            # is in alternateUrl, rather than in url?
                            # print(f"alternateUrl {row[alternateUrl]} needs to be scraped for ApiURLs?")
                            ApiURLs = self.GetStaticUrls(row['alternateUrl'])
                else:
                    # URL is for a static page that needs to be scraped to find
                    # the signupgenius urls. Sometimes there is a mistake in airtable
                    # and they put the signupgenius url in URL instead of alternateUrl,
                    # so check for that first.
                    if 'signupgenius.com' in URL:
                        ApiURLs.append(URL)
                    else:
                        # Scrape the static page to get the urls.
                        ApiURLs = self.GetStaticUrls(URL)

                # print(f"ApiURLs: {ApiURLs}")
                scraper = SignUpGeniusBaseGeneralized(ApiURLs, LocationName, [Keys])
                keys, case, text = scraper.MakeGetRequest()
                print(keys, case)

            except Exception as e:
                print(f"Ran into error {e} processing {LocationName} in SignUpGeniusWrapperGeneralized.")

if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)
    scraper_wrapper = SignUpGeniusWrapperGeneralized()
    scraper_wrapper.MakeGetRequest()
