import requests
from bs4 import BeautifulSoup
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable
import logging
from DummyScraper import DummyScraper

class PrepModSites():

    def __init__(self):
        self.URL = "https://prepmod.doh.wa.gov/clinic/search?page="
        self.site_dict = {
            'prepmod_consistent_care':0,
            'prepmod_delta_direct':0,
            'prepmod_island_drug':0,
            'prepmod_angel_of':0,
            'prepmod_town_toyota':0,
            'prepmod_benton_franklin':0,
            'prepmod_evergreen_state':0,
            'prepmod_skagit_county':0,
            'prepmod_yakima_county':0,
            'prepmod_lacey_family':0,
            'prepmod_centralia_internal':0,
            'prepmod_walla_walla':0,
            'prepmod_nouveau_medspas':0,
            'prepmod_samaritan_family':0,
            'prepmod_snohomish_county':0,
            'prepmod_whatcom_county':0,
            'prepmod_la_conner':0,
            'prepmod_centralia_internal':0,
            'prepmod_thurston_county':0,
            'prepmod_evergreen_internists':0,
        }

    def MakeGetRequest(self):
        html  = ""
        listItems = []
        # scrape first 5 pages of url
        for i in range(1,6):
            r = requests.get(self.URL+str(i))
            html +=r.text
            soup = BeautifulSoup(r.content, 'html.parser')
            listItems+=soup.find_all("div", class_="md:flex-shrink text-gray-800")

        raw_clinic = []
        for element in listItems:
            ps = element.find_all("p")

            name = ps[0].text[7:-19]

            # Craft up a key to use instead of name

            words = name.lower().split()
            key = "prepmod_" + words[0]
            if len(words) > 1:
                key += "_" + words[1]

            vac = ""
            for p in ps:
                for strong_tag in p.find_all('strong'):
                    if strong_tag.text.startswith("Available Appointments"):
                        vac = p.text
                        break

            v = int(vac[33:]) if vac!="" else 0
            raw_clinic.append((key,v))


        for key, vac in raw_clinic:
            if key in self.site_dict.keys():
                self.site_dict[key] += vac
            else:
                self.site_dict[key] = vac

        ### call dummy scraper for all data collected
        for key in self.site_dict:
            case = Status.YES if self.site_dict[key]>0 else Status.NO
            scraper = DummyScraper(keys=[key],case = case, html = html)
            scraper.MakeGetRequest()

        return "prepmod", 0, html

if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)
    scraper = PrepModSites()
    keys, case, text = scraper.MakeGetRequest()
    logging.debug(f"keys={keys} case={case} ")
