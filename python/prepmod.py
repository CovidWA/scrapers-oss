###################################################################################################################################################
#The prepmod class acts as a way to check prepmod links for availability by checking for two negative cases (deadline and closure)
#The prepmod class also checks for the positive case when the table shows up. It checks through the table as well to make sure
#that the table contains at least on available appointment. prepmod takes a list of the links to the disired prepmod pages as a pamerter
#and returns the combined status of the links (if one link has appointment then case=Status.YES.
#__________________________________________________________________________________________________________________________________________________
# x = prepmod(prepmod_links) creates the prepmod checking object
# x.getcase() returns the combined status of the links
# see EvergreenState.py as an example



import requests
from bs4 import BeautifulSoup
from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable
from typing import Tuple, List
import logging

class prepmod(ScraperBase):


    def __init__(self, prepmod_links, failure_str=""):
        self.prepmod_links = prepmod_links

        self.FailureCase = failure_str if failure_str else "Deadline to register for this clinic has been reached. Please check other clinics."

        self.SuccessCase = "Please select a time for your appointment"

    #gets status of combined links
    def getcase(self):
        case = Status.POSSIBLE       # default to possible
        failures = 0                 # count how many prepmod links failed
        #wait_overall = 0  part potential add

        for url in self.prepmod_links:
            r = requests.get(url)
            # Parse the HTML on prepmod site
            soup = BeautifulSoup(r.content, 'html.parser')

            # parse for the success case - could be simplified to remove
            # redundant work, but will leave for now
            listItems = soup.find_all('p')
            for element in listItems:

                # see if success case is found in the paragraph
                if str(element).find(self.SuccessCase) != -1:
                    case = Status.YES
                    break

            # parse for the failure case
            div = soup.find('div', class_ = "danger-alert")
            #covers when page has failure case
            if(div):
                #checks text for failure case
                if (self.FailureCase in div.text or "This clinic is closed. Please check other clinics." in div.text or "Clinic does not have any appointment slots available" in div.text):
                    #Still a no, flag it
                    failures += 1
            #covers the case when the link goes to table of appointments
            else:
                #priming counts of no appointments
                negatives = 0
                waitlist = 0
                #finds all tables on page(always two)
                table = soup.find("table")
                tb = table.find("tbody")
                #grabs rows
                table_rows = tb.find_all("tr")
                #checks each row for failure case
                for tr in table_rows:
                    td = tr.find("td")
                    if "No" in td.text:
                        #counts no appointment cases in table
                        negatives+=1
                    #counts wait list rows set up to allow us to notice waitlist
                    elif "Someone will contact you about your appointment." in td.text:
                        waitlist+=1
                #checks that No appointments are available in the table
                if negatives+waitlist == len(table_rows):
                    failures +=1


        # all the prepmod links are unavailable - exit
        if failures == len(self.prepmod_links):
            #potential waitlist add
            #if wait_overall > 0:
            #   case = Status.WAITLIST
            case = Status.NO

        return case
