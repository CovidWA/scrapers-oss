#!/usr/bin/env python
#from AuburnShoWare import AuburnScraper
# from EvergreenHospital import EvergreenHospital
# from UWScraper import UWScraper

from PrepModSearch import PrepModSearch

# FredHutch not correct
# from FredHutchScraper import FredHutchScraper
# from ScraperBase import ScraperBase
# from ConfluenceHealth import ConfluenceHealth
# from FerryCountyHealth import FerryCountyHealth
# EvergreenIntern now scraped by Go
# from EvergreenIntern import EvergreenIntern

# Moved to Typescript
# from EvergreenState import EvergreenState
# from UniversityPlaceClinic import UniversityPlaceClinic
# Bainbridge moved to TS timetap scraper.
# from BainbridgeIslandSeniorCenter import BainbridgeIslandSeniorCenter

# @mazore scrapers
# Kattermans moved to golang
#from Kattermans import Kattermans
# from RiteAidAlternate import RiteAidAlternate
from HealthMartPharmacies import HealthMartPharmacies

# @deathbymochi scrapers
from PCHSGatewayCenter import PCHSGatewayCenter
from PCHSNorthMasonGym import PCHSNorthMasonGym

# @richtong scrapers
# from GraysHarborCounty import GraysHarborCounty
# from KlickitatValleyHealth import KlickitatValleyHealth
# from SidsPharmacy import SidsPharmacy

# this is not static HTML so requires Selenium to go through it
# no information online, you have to call so leave as possible
# from UnifyCommunityMission import UnifyCommunityMission
from SeattleIDC import SeattleIDC
# from SignUpGeniusScraperWrapper import SignUpGeniusScraperWrapper

# @skipper8 scrapers
from CamanoFire import CamanoFire #timsliu too
# from EvergreenstatePrepmod import EvergreenstatePrepmod
# from Mason_Count import Mason_Count
from SanJuan import SanJuan
from SanJuanOrca import SanJuanOrca
from SanJuanLopez import SanJuanLopez

# @leesmith scrapers
# MemberPlus moved to TS timetap scraper.
#from MemberPlusFamilyHealth import MemberPlusFamilyHealth
from PreventionNorthwest import PreventionNorthwest
from CascadeMedical import CascadeMedical
# from IslandDrugPrepMod import IslandDrugWrapper
from CascadeHealthClinic import CascadeHealthClinic
from GraysHarborCounty import GraysHarborCounty
from FridayHarborDrug import FridayHarborDrug
from SignUpGeniusWrapperGeneralized import SignUpGeniusWrapperGeneralized


# @linhchan scrapers
# CamanoIslandHealthSystem now scraped by Go
# from CamanoIslandHealthSystem import CamanoIslandHealthSystem
# from NooksackValleyDrugstore import NooksackValleyDrugstore

# shepwalker scrapers
#from WalgreensScraperWrapper import WalgreensScraperWrapper

# timsliu scrapers
# from YakimaValley import YakimaValley
# Kitsap moved to TS timetap scraper.
# from KitsapCommunityClinic import KitsapCommunityClinic
#from FamilyCareNetwork import FamilyCareNetworkWrapper
from AcuityScheduling import AcuitySchedulingWrapper
# from PharmacaWrapper import PharmacaWrapper
from MtSpokanePediatrics import MtSpokanePediatrics

# @Myau5x scrapers
# from MasonGeneral import MasonGeneral
from SeaMarScraper import SeaMarScraper
# from OnScene import OnScene
# from LakeChelan import LakeChelan
# from PrepModSites import PrepModSites

# import requests
import logging

# import json
# import os
import argparse
import traceback

# tazadejava costco scraper
# from Costco import CostcoWrapper

class ScrapeAllAndSend:
    def __init__(self):
        self.lstActiveScrapers = [
            # YakimaValley(),
            # UniversityPlaceClinic(),
            # FerryCountyHealth(),
            HealthMartPharmacies(),
            # ConfluenceHealth(),
            # UWScraper(),
            # EvergreenHospital(),
            GraysHarborCounty(),
            # KlickitatValleyHealth(),
            PCHSGatewayCenter(),
            PCHSNorthMasonGym(),
            # SidsPharmacy(),
            # UnifyCommunityMission(),
            SeattleIDC(),
            # SignUpGeniusScraperWrapper(),
            CamanoFire(),
            PreventionNorthwest(),
            # MasonGeneral(),
            SeaMarScraper(),
            # OnScene(),
            # LakeChelan(),
            # PrepModSites(),
            # NooksackValleyDrugstore(),
            AcuitySchedulingWrapper(),
            # PharmacaWrapper(),
            # EvergreenstatePrepmod(),
            # EvergreenState(),
            CascadeMedical(),
            PrepModSearch(),
            # Mason_Count(),
            # IslandDrugWrapper(),
            SanJuan(),
            SanJuanOrca(),
            SanJuanLopez(),
            CascadeHealthClinic(),
            # CostcoWrapper(),
            MtSpokanePediatrics(),
            FridayHarborDrug(),
            SignUpGeniusWrapperGeneralized(),
        ]

        scraper_types = [type(scraper).__name__ for scraper in self.lstActiveScrapers]
        logging.info("Beginning Scrape of: " + ', '.join(scraper_types))

    def beginScrape(self):
        for scraper in self.lstActiveScrapers:
            try:
                scraper.MakeGetRequest()
            except Exception as e:

                logging.error(f"{scraper.__class__.__name__} had an exception {e}\n{traceback.format_exc()}")


def handler(event, context):
    try:
        logging.basicConfig(level=logging.DEBUG)
        logging.debug("ScrapeAllAndSend started")
        scrape = ScrapeAllAndSend()
        scrape.beginScrape()
    except Exception as e:
        logging.error(e)
        raise Exception("Error occurred during execution")
    return {"statusCode": 200}


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument('-t','--test', help='Test Individual Scraper',required=False)
    args = parser.parse_args()
    if args.test != None:
        print(f'Testing scraper: {args.test}')
        test_scraper = eval(args.test + "()")
        test_scraper.MakeGetRequest()
    else:
        handler(None, None)
