import csv
from WalgreensScraper import WalgreensScraper
class WalgreensScraperWrapper():
    def __init__(self):
        self.WalgreensList = 'WalgreensCities.csv'
        self.KeyPrefix = "Walgreens"
    def MakeGetRequest(self):
        with open(self.WalgreensList, mode='r') as csv_file:
            csv_reader = csv.reader(csv_file, delimiter=',')
            for row in csv_reader:
                if(len(row) > 2):
                    try:
                        print("Processing line " + row[0])
                        name = row[0]
                        latitude = float(row[1])
                        longitude = float(row[2])
                        scraper = WalgreensScraper(latitude, longitude, name, [self.KeyPrefix + name])
                        scraper.MakeGetRequest()
                    except Exception as e:
                        print("Ran into error processing row in WalgreensWrapper." + str(e))