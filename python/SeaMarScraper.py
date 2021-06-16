import requests
from bs4 import BeautifulSoup
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable, GetClinicsData, VaccineType
import logging
from DummyScraper import DummyScraper

class SeaMarScraper():

    def __init__(self):
        self.URL = "https://docs.google.com/spreadsheets/d/e/2PACX-1vRbT-9tFmezrnQ6ETYmF81PDcqgHg5eDD4Xva8krr9GiELl9fFPMhuUvm5WH-qsU1jf-FEcSQKhVM10/pubhtml"

    def MakeGetRequest(self):

        r = requests.get(self.URL)
        html = r.text

        # Read the Sea Mar clinics out of airtable, initialize case to NO.
        siteOutputDict={}
        siteInputDict = GetClinicsData(key_filter='Sea Mar ')
        for site in siteInputDict:
            key = site['key']
            siteOutputDict[key] = DummyScraper([key], Status.NO, html)
        dictLength = len(siteOutputDict)

        # Parse the HTML
        soup = BeautifulSoup(r.content, 'html.parser')
        listItems = soup.find_all('tr')

        # Skip over the first table rows. Loop a max of
        # dictLength times, but will break out sooner in the
        # case where clinics aren't reporting in the googledoc.
        for i in range(5, dictLength + 5):
            x = listItems[i].select("td")
            if not x:
                break

            #cl = {"Name": "Sea Mar "+x[0].text, "Vaccine": x[1].text}
            key = "Sea Mar "+x[0].text
            if x[1].text == "Vaccines Available on":
                vaccTypes = set()
                typeColumn = x[3].text
                if "Moderna" in typeColumn:
                    vaccTypes.add(VaccineType.MODERNA.value)
                if "Pfizer" in typeColumn:
                    vaccTypes.add(VaccineType.PFIZER.value)
                # N.B. We don't know what J&J would look like yet.

                siteOutputDict[key] = DummyScraper([key], Status.WALKIN, html, vaccTypes)

        # Now update airtable
        for key in siteOutputDict:
            siteOutputDict[key].MakeGetRequest()

        return siteOutputDict

if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)
    scraper = SeaMarScraper()
    siteOutputDict = scraper.MakeGetRequest()

    # for key in siteOutputDict:
    #     logging.info(siteOutputDict[key])
