import requests
from bs4 import BeautifulSoup
from ScraperBase import ScraperBase
from Common import Status, ForcePushResultDictToDB, GetClinicsData, VaccineType
from PrepModResult import PrepModResult
import logging

class PrepModSearch():

    def __init__(self):
        # Old URL
        # self.URL = 'https://prepmod.doh.wa.gov/clinic/search?location=&search_radius=All&q[venue_search_name_or_venue_name_i_cont]=&q[vaccinations_name_i_cont]=&clinic_date_eq[year]=&clinic_date_eq[month]=&clinic_date_eq[day]=&commit=Search&page={}#search_results'

        # new URL as of 5/9/21
        self.URL = 'https://prepmod.doh.wa.gov/appointment/en/clinic/search?location=&search_radius=All&q[venue_search_name_or_venue_name_i_cont]=&q[vaccinations_name_i_cont]=&clinic_date_eq[year]=&clinic_date_eq[month]=&clinic_date_eq[day]=&commit=Search&page={}#search_results'

        # Dictionary used to map zip code to county. If county is "", that means
        # that zip code could be in 1 of 2-3 different counties. Null string will
        # not overwrite the county cell in airtable. Testers will manually fix.

        self.zipToCountyDict = {
        "98001": "King County", "98002": "King County", "98003": "King County", "98004": "King County",
        "98005": "King County", "98006": "King County", "98007": "King County", "98008": "King County", "98009": "King County",
        "98010": "King County", "98011": "", "98012": "Snohomish County", "98013": "King County", "98014": "King County",
        "98015": "King County", "98019": "", "98020": "Snohomish County", "98021": "Snohomish County", "98022": "", "98023": "",
        "98024": "King County", "98025": "King County", "98026": "Snohomish County", "98027": "King County", "98028": "",
        "98029": "King County", "98030": "King County", "98031": "King County", "98032": "King County", "98033": "King County",
        "98034": "King County", "98035": "King County", "98036": "Snohomish County", "98037": "Snohomish County",
        "98038": "King County", "98039": "King County", "98040": "King County", "98041": "King County", "98042": "King County",
         "98043": "Snohomish County", "98045": "King County", "98046": "Snohomish County", "98047": "", "98050": "King County",
         "98051": "King County", "98052": "King County", "98053": "King County", "98054": "King County", "98055": "King County",
         "98056": "King County", "98057": "King County", "98058": "King County", "98059": "King County", "98061": "Kitsap County",
         "98062": "King County", "98063": "King County", "98064": "King County", "98065": "King County", "98068": "",
         "98070": "King County", "98071": "King County", "98072": "", "98073": "King County", "98074": "King County",
         "98075": "King County", "98077": "", "98082": "", "98083": "King County", "98087": "Snohomish County",
         "98089": "King County", "98092": "", "98093": "King County", "98101": "King County", "98102": "King County",
         "98103": "King County", "98104": "King County", "98105": "King County", "98106": "King County", "98107": "King County",
         "98108": "King County", "98109": "King County", "98110": "", "98111": "King County", "98112": "King County",
         "98113": "King County", "98114": "King County", "98115": "King County", "98116": "King County", "98117": "King County",
         "98118": "King County", "98119": "King County", "98121": "King County", "98122": "King County", "98124": "King County",
         "98125": "King County", "98126": "King County", "98127": "King County", "98129": "King County", "98131": "King County",
         "98132": "King County", "98133": "King County", "98134": "King County", "98136": "King County", "98138": "King County",
         "98139": "King County", "98141": "King County", "98144": "King County", "98145": "King County", "98146": "King County",
         "98148": "King County", "98151": "King County", "98154": "King County", "98155": "King County", "98158": "King County",
         "98160": "King County", "98161": "King County", "98164": "King County", "98165": "King County", "98166": "King County",
         "98168": "King County", "98170": "King County", "98171": "King County", "98174": "King County", "98175": "King County",
         "98177": "", "98178": "King County", "98181": "King County", "98184": "King County", "98185": "King County",
         "98188": "King County", "98189": "", "98190": "King County", "98191": "King County", "98194": "King County",
         "98195": "King County", "98198": "King County", "98199": "King County", "98201": "Snohomish County",
         "98203": "Snohomish County", "98204": "Snohomish County", "98205": "Snohomish County", "98206": "Snohomish County",
         "98207": "Snohomish County", "98208": "Snohomish County", "98213": "Snohomish County", "98220": "Whatcom County",
         "98221": "Skagit County", "98222": "San Juan County", "98223": "Snohomish County", "98224": "King County",
         "98225": "Whatcom County", "98226": "Whatcom County", "98227": "Whatcom County", "98228": "Whatcom County",
         "98229": "", "98230": "Whatcom County", "98231": "Whatcom County", "98232": "Skagit County", "98233": "Skagit County",
         "98235": "Skagit County", "98236": "Island County", "98237": "", "98238": "Skagit County", "98239": "Island County",
         "98240": "Whatcom County", "98241": "", "98243": "San Juan County", "98244": "Whatcom County", "98245": "San Juan County",
         "98247": "Whatcom County", "98248": "Whatcom County", "98249": "Island County", "98250": "San Juan County", "98251": "",
         "98252": "Snohomish County", "98253": "Island County", "98255": "Skagit County", "98256": "Snohomish County",
         "98257": "Skagit County", "98258": "Snohomish County", "98259": "Snohomish County", "98260": "Island County",
         "98261": "San Juan County", "98262": "Whatcom County", "98263": "Skagit County", "98264": "Whatcom County",
         "98266": "Whatcom County", "98267": "Skagit County", "98270": "Snohomish County", "98271": "Snohomish County",
         "98272": "Snohomish County", "98273": "Skagit County", "98274": "Skagit County", "98275": "Snohomish County",
         "98276": "Whatcom County", "98277": "Island County", "98278": "Island County", "98279": "San Juan County",
         "98280": "San Juan County", "98281": "Whatcom County", "98282": "", "98283": "", "98284": "", "98286": "San Juan County",
         "98287": "Snohomish County", "98288": "King County", "98290": "Snohomish County", "98291": "Snohomish County", "98292": "",
         "98293": "Snohomish County", "98294": "Snohomish County", "98295": "Whatcom County", "98296": "Snohomish County",
         "98297": "San Juan County", "98303": "Pierce County", "98304": "", "98305": "Clallam County", "98310": "Kitsap County",
         "98311": "Kitsap County", "98312": "", "98314": "Kitsap County", "98315": "Kitsap County", "98320": "Jefferson County",
         "98321": "Pierce County", "98322": "Kitsap County", "98323": "Pierce County", "98324": "Clallam County",
         "98325": "Jefferson County", "98326": "Clallam County", "98327": "Pierce County", "98328": "Pierce County", "98329": "",
         "98330": "", "98331": "", "98332": "Pierce County", "98333": "Pierce County", "98335": "Pierce County",
         "98336": "Lewis County", "98337": "Kitsap County", "98338": "Pierce County", "98339": "Jefferson County",
         "98340": "Kitsap County", "98342": "Kitsap County", "98343": "Clallam County", "98344": "Pierce County",
         "98345": "Kitsap County", "98346": "Kitsap County", "98348": "Thurston County", "98349": "Pierce County",
         "98350": "Clallam County", "98351": "Pierce County", "98352": "Pierce County", "98353": "Kitsap County", "98354": "",
         "98355": "Lewis County", "98356": "Lewis County", "98357": "Clallam County", "98358": "Jefferson County", "98359": "",
         "98360": "Pierce County", "98361": "Lewis County", "98362": "Clallam County", "98363": "Clallam County",
         "98364": "Kitsap County", "98365": "Jefferson County", "98366": "Kitsap County", "98367": "Kitsap County",
         "98368": "Jefferson County", "98370": "Kitsap County", "98371": "Pierce County", "98372": "Pierce County",
         "98373": "Pierce County", "98374": "Pierce County", "98375": "Pierce County", "98376": "Jefferson County",
         "98377": "Lewis County", "98378": "Kitsap County", "98380": "", "98381": "Clallam County", "98382": "",
         "98383": "Kitsap County", "98384": "Kitsap County", "98385": "Pierce County", "98386": "Kitsap County",
         "98387": "Pierce County", "98388": "Pierce County", "98390": "Pierce County", "98391": "Pierce County",
         "98392": "Kitsap County", "98393": "Kitsap County", "98394": "Pierce County", "98395": "Pierce County",
         "98396": "Pierce County", "98397": "Pierce County", "98398": "Lewis County", "98401": "Pierce County",
         "98402": "Pierce County", "98403": "Pierce County", "98404": "Pierce County", "98405": "Pierce County",
         "98406": "Pierce County", "98407": "Pierce County", "98408": "Pierce County", "98409": "Pierce County",
         "98411": "Pierce County", "98412": "Pierce County", "98413": "Pierce County", "98415": "Pierce County",
         "98416": "Pierce County", "98417": "Pierce County", "98418": "Pierce County", "98419": "Pierce County",
         "98421": "Pierce County", "98422": "", "98424": "Pierce County", "98430": "Pierce County", "98431": "Pierce County",
         "98433": "Pierce County", "98438": "Pierce County", "98439": "Pierce County", "98442": "Pierce County",
         "98443": "Pierce County", "98444": "Pierce County", "98445": "Pierce County", "98446": "Pierce County",
         "98447": "Pierce County", "98448": "Pierce County", "98450": "Pierce County", "98455": "Pierce County",
         "98460": "Pierce County", "98464": "Pierce County", "98465": "Pierce County", "98466": "Pierce County",
         "98467": "Pierce County", "98471": "Pierce County", "98477": "Pierce County", "98481": "Pierce County",
         "98490": "Pierce County", "98492": "Pierce County", "98493": "Pierce County", "98496": "Pierce County",
         "98497": "Pierce County", "98498": "Pierce County", "98499": "Pierce County", "98501": "Thurston County",
         "98502": "", "98503": "Thurston County", "98504": "Thurston County", "98505": "Thurston County", "98506": "Thurston County",
         "98507": "Thurston County", "98508": "Thurston County", "98509": "Thurston County", "98511": "Thurston County",
         "98512": "Thurston County", "98513": "Thurston County", "98516": "Thurston County", "98520": "Grays Harbor County",
         "98522": "Lewis County", "98524": "Mason County", "98526": "Grays Harbor County", "98527": "Pacific County", "98528": "",
         "98530": "Thurston County", "98531": "", "98532": "Lewis County", "98533": "Lewis County", "98535": "Grays Harbor County",
         "98536": "Grays Harbor County", "98537": "", "98538": "Lewis County", "98539": "Lewis County", "98540": "Thurston County",
         "98541": "", "98542": "Lewis County", "98544": "Lewis County", "98546": "Mason County", "98547": "", "98548": "Mason County",
         "98550": "Grays Harbor County", "98552": "Grays Harbor County", "98554": "Pacific County", "98555": "Mason County",
         "98556": "Thurston County", "98557": "", "98558": "Pierce County", "98559": "Grays Harbor County", "98560": "Mason County",
         "98561": "Pacific County", "98562": "Grays Harbor County", "98563": "", "98564": "Lewis County", "98565": "Lewis County",
         "98566": "Grays Harbor County", "98568": "", "98569": "Grays Harbor County", "98570": "Lewis County",
         "98571": "Grays Harbor County", "98572": "", "98575": "Grays Harbor County", "98576": "Thurston County",
         "98577": "Pacific County", "98579": "", "98580": "Pierce County", "98581": "Cowlitz County", "98582": "Lewis County",
         "98583": "Grays Harbor County", "98584": "Mason County", "98585": "Lewis County", "98586": "Pacific County",
         "98587": "Grays Harbor County", "98588": "Mason County", "98589": "Thurston County", "98590": "Pacific County",
         "98591": "Lewis County", "98592": "Mason County", "98593": "Lewis County", "98595": "Grays Harbor County",
         "98596": "Lewis County", "98597": "Thurston County", "98599": "Thurston County", "98601": "", "98602": "Klickitat County",
         "98603": "Cowlitz County", "98604": "Clark County", "98605": "", "98606": "Clark County", "98607": "Clark County",
         "98609": "Cowlitz County", "98610": "Skamania County", "98611": "Cowlitz County", "98612": "Wahkiakum County",
         "98613": "Klickitat County", "98614": "Pacific County", "98616": "", "98617": "Klickitat County", "98619": "Klickitat County",
         "98620": "Klickitat County", "98621": "Wahkiakum County", "98622": "Clark County", "98623": "Klickitat County",
         "98624": "Pacific County", "98625": "Cowlitz County", "98626": "Cowlitz County", "98628": "Klickitat County",
         "98629": "Clark County", "98631": "Pacific County", "98632": "", "98635": "Klickitat County", "98637": "Pacific County",
         "98638": "", "98639": "Skamania County", "98640": "Pacific County", "98641": "Pacific County", "98642": "Clark County",
         "98643": "Wahkiakum County", "98644": "Pacific County", "98645": "Cowlitz County", "98647": "Wahkiakum County",
         "98648": "Skamania County", "98649": "Cowlitz County", "98650": "Klickitat County", "98651": "", "98660": "Clark County",
         "98661": "Clark County", "98662": "Clark County", "98663": "Clark County", "98664": "Clark County", "98665": "Clark County",
         "98666": "Clark County", "98667": "Clark County", "98668": "Clark County", "98670": "Klickitat County", "98671": "",
         "98672": "", "98673": "Klickitat County", "98674": "", "98675": "Clark County", "98682": "Clark County",
         "98683": "Clark County", "98684": "Clark County", "98685": "Clark County", "98686": "Clark County", "98687": "Clark County",
         "98801": "Chelan County", "98802": "Douglas County", "98807": "Chelan County", "98811": "Chelan County",
         "98812": "Okanogan County", "98813": "Douglas County", "98814": "Okanogan County", "98815": "Chelan County",
         "98816": "Chelan County", "98817": "Chelan County", "98819": "Okanogan County", "98821": "Chelan County",
         "98822": "Chelan County", "98823": "Grant County", "98824": "Grant County", "98826": "Chelan County",
         "98827": "Okanogan County", "98828": "Chelan County", "98829": "Okanogan County", "98830": "Douglas County",
         "98831": "Chelan County", "98832": "Grant County", "98833": "Okanogan County", "98834": "Okanogan County",
         "98836": "Chelan County", "98837": "Grant County", "98840": "Okanogan County", "98841": "Okanogan County",
         "98843": "Douglas County", "98844": "Okanogan County", "98845": "Douglas County", "98846": "Okanogan County",
         "98847": "Chelan County", "98848": "Grant County", "98849": "Okanogan County", "98850": "Douglas County",
         "98851": "Grant County", "98852": "Chelan County", "98853": "Grant County", "98855": "Okanogan County",
         "98856": "Okanogan County", "98857": "Grant County", "98858": "Douglas County", "98859": "Okanogan County",
         "98860": "Grant County", "98862": "Okanogan County", "98901": "Yakima County", "98902": "Yakima County",
         "98903": "Yakima County", "98904": "Yakima County", "98907": "Yakima County", "98908": "Yakima County",
         "98909": "Yakima County", "98920": "Yakima County", "98921": "Yakima County", "98922": "Kittitas County",
         "98923": "Yakima County", "98925": "Kittitas County", "98926": "Kittitas County", "98929": "Yakima County",
         "98930": "Yakima County", "98932": "Yakima County", "98933": "Yakima County", "98934": "Kittitas County",
         "98935": "Yakima County", "98936": "Yakima County", "98937": "Yakima County", "98938": "Yakima County",
         "98939": "Yakima County", "98940": "Kittitas County", "98941": "Kittitas County", "98942": "Yakima County",
         "98943": "Kittitas County", "98944": "Yakima County", "98946": "Kittitas County", "98947": "Yakima County",
         "98948": "Yakima County", "98950": "Kittitas County", "98951": "Yakima County", "98952": "Yakima County",
         "98953": "Yakima County", "99001": "Spokane County", "99003": "Spokane County", "99004": "Spokane County",
         "99005": "Spokane County", "99006": "Spokane County", "99008": "Lincoln County", "99009": "Spokane County",
         "99011": "Spokane County", "99012": "Spokane County", "99013": "Stevens County", "99014": "Spokane County",
         "99016": "Spokane County", "99017": "Whitman County", "99018": "Spokane County", "99019": "Spokane County",
         "99020": "Spokane County", "99021": "Spokane County", "99022": "Spokane County", "99023": "Spokane County",
         "99025": "Spokane County", "99026": "Stevens County", "99027": "Spokane County", "99029": "Lincoln County",
         "99030": "Spokane County", "99031": "Spokane County", "99032": "Lincoln County", "99033": "Whitman County",
         "99034": "Stevens County", "99036": "Spokane County", "99037": "Spokane County", "99039": "Spokane County",
         "99040": "Stevens County", "99101": "Stevens County", "99102": "Whitman County", "99103": "Lincoln County",
         "99104": "Whitman County", "99105": "Adams County", "99107": "Ferry County", "99109": "Stevens County",
         "99110": "Stevens County", "99111": "Whitman County", "99113": "Whitman County", "99114": "Stevens County",
         "99115": "Grant County", "99116": "Douglas County", "99117": "Lincoln County", "99118": "Ferry County",
         "99119": "Pend Oreille County", "99121": "Ferry County", "99122": "Lincoln County", "99123": "Grant County",
         "99124": "Okanogan County", "99125": "Whitman County", "99126": "Stevens County", "99128": "Whitman County",
         "99129": "Stevens County", "99130": "Whitman County", "99131": "Stevens County", "99133": "Grant County",
         "99134": "Lincoln County", "99135": "Grant County", "99136": "Whitman County", "99137": "Stevens County",
         "99138": "Ferry County", "99139": "Pend Oreille County", "99140": "Ferry County", "99141": "Stevens County",
         "99143": "Whitman County", "99144": "Lincoln County", "99146": "Ferry County", "99147": "Lincoln County",
         "99148": "Stevens County", "99149": "Whitman County", "99150": "Ferry County", "99151": "Stevens County",
         "99152": "Pend Oreille County", "99153": "Pend Oreille County", "99154": "Lincoln County", "99155": "Okanogan County",
         "99156": "Pend Oreille County", "99157": "Stevens County", "99158": "Whitman County", "99159": "Lincoln County",
         "99160": "Ferry County", "99161": "Whitman County", "99163": "Whitman County", "99164": "Whitman County",
         "99165": "Whitman County", "99166": "Ferry County", "99167": "Stevens County", "99169": "Adams County", "99170": "",
         "99171": "Whitman County", "99173": "Stevens County", "99174": "Whitman County", "99176": "Whitman County",
         "99179": "Whitman County", "99180": "Pend Oreille County", "99181": "Stevens County", "99185": "Lincoln County",
         "99201": "Spokane County", "99202": "Spokane County", "99203": "Spokane County", "99204": "Spokane County",
         "99205": "Spokane County", "99206": "Spokane County", "99207": "Spokane County", "99208": "Spokane County",
         "99209": "Spokane County", "99210": "Spokane County", "99211": "Spokane County", "99212": "Spokane County",
         "99213": "Spokane County", "99214": "Spokane County", "99215": "Spokane County", "99216": "Spokane County",
         "99217": "Spokane County", "99218": "Spokane County", "99219": "Spokane County", "99220": "Spokane County",
         "99223": "Spokane County", "99224": "Spokane County", "99228": "Spokane County", "99251": "Spokane County",
         "99252": "Spokane County", "99256": "Spokane County", "99258": "Spokane County", "99260": "Spokane County",
         "99299": "Spokane County", "99301": "Franklin County", "99302": "Franklin County", "99320": "Benton County",
         "99321": "Grant County", "99322": "", "99323": "Walla Walla County", "99324": "Walla Walla County", "99326": "",
         "99328": "", "99329": "Walla Walla County", "99330": "Franklin County", "99333": "Whitman County", "99335": "Franklin County",
         "99336": "Benton County", "99337": "Benton County", "99338": "Benton County", "99341": "Adams County",
         "99343": "Franklin County", "99344": "", "99345": "Benton County", "99346": "Benton County", "99347": "", "99348": "",
         "99349": "Grant County", "99350": "", "99352": "Benton County", "99353": "Benton County", "99354": "Benton County",
         "99356": "Klickitat County", "99357": "Grant County", "99359": "Columbia County", "99360": "Walla Walla County",
         "99361": "", "99362": "Walla Walla County", "99363": "Walla Walla County", "99371": "", "99401": "Asotin County",
         "99402": "Asotin County", "99403": ""
         }

    @ForcePushResultDictToDB
    def MakeGetRequest(self):
        siteOutputDict={}
        siteInputDict = GetClinicsData(key_filter='autoprepmod')
        for site in siteInputDict:
            key = site['key']
            siteOutputDict[key] = PrepModResult(key, Status.NO, site['name'], \
                    site['address'], site['url'], "", "", 0, set())

        CurrentPageInt = 1

        #Get number of pages to scrape
        r = requests.get(self.URL.format(CurrentPageInt))
        soup = BeautifulSoup(r.content, 'html.parser')
        pages = soup.find_all("span", class_="page")
        numberActivePages = (int) (pages[len(pages)-2].text)

        #get sites from first page (reuse request)
        sitesList = soup.find_all("div", class_="md:flex-shrink text-gray-800")
        CurrentPageInt +=1

        #get sites from all pages
        for CurrentPageInt in range(CurrentPageInt, (numberActivePages + 1)):
          urlToCall = self.URL.format(CurrentPageInt)
          r = requests.get(urlToCall)
          soup = BeautifulSoup(r.content, 'html.parser')
          sitesList += soup.find_all("div", class_="md:flex-shrink text-gray-800")

        #Iterate through all sites detected
        for siteEntry in sitesList:
          SitePElements = siteEntry.find_all("p")

          #Extract SiteName from title of format 'SiteName on xx/xx/xxxx'
          nameAndDate = siteEntry.h2.text.strip()
          name = nameAndDate.split(" on ")[0].strip()

          # Extract vaccine type(s) so we can append to name.
          vaccineTypes = \
            SitePElements[1].select("p > strong:nth-of-type(2)")[0].get_text().strip().replace('-', ' ')
          # print(f"{name} has vaccine type(s): {vaccineTypes}")

          # Append vaccine type(s) to name. Change Janssen to J&J. Disallow non-vaccine-related
          # text (e.g., "Conact site" instead of Pfizer/Moderna/Janssen). Set scraperTags for
          # the vaccine type(s).
          vaccList = [vc[0] for vc in (v.split(' ', 1) for v in vaccineTypes.split(', '))]
          # print(f"vaccList: {vaccList}")

          scraperTags = set()   # Initialize to empty Set

          if any(vc.lower() in ["pfizer", "moderna", "janssen", "j&j"] for vc in vaccList):
              # Add vaccine type(s) to name
              vaccName = '/'.join([vc for vc in vaccList]).replace('Janssen', 'J&J')
              name += ' (' + vaccName + ')'

              # Add scraperTags for vaccine type(s)
              if 'Pfizer' in vaccName:
                  scraperTags.add(VaccineType.PFIZER.value)
              if 'Moderna' in vaccName:
                  scraperTags.add(VaccineType.MODERNA.value)
              if 'J&J' in vaccName:
                  scraperTags.add(VaccineType.JOHNSON.value)

          # print(f"scraperTags: {scraperTags} for {name}")

          #Extract address and generate consistent key based on address
          address = SitePElements[0].text.strip()
          key = self.generateKeyFromAddress(address)

          # Get availability count
          vaccineAvailabilityCount = self.checkIfSiteEntryHasAvailability(SitePElements)
          # print(f"{name}: count = {vaccineAvailabilityCount}")

          # If availability count > 0, still need to make sure there is a booking button,
          # before changing case to YES, otherwise case = NO. If NO, make sure
          # availability count gets reset to zero for PrepModResult() handling below.

          case = Status.NO
          if (vaccineAvailabilityCount > 0):
              SiteButtonElement = siteEntry.find("a", class_="button-primary px-4")
              if (SiteButtonElement):
                  if ("sign up for a covid" in SiteButtonElement.text.lower()):
                      case = Status.YES
                  else:
                      vaccineAvailabilityCount = 0
                      logging.debug(f"Booking button found, but text does not match")
              else:
                  vaccineAvailabilityCount = 0
                  logging.debug(f"Availability > 0 but no booking button, return NO")

          #Get the link for passthrough
          content_url = self.generateURLForSite(name)
          if key in siteOutputDict:
            # Entry already exists in siteOutputDict, but we update it
            # with new name (which now has vaccine type appended) and avail_count.
            # In the NO case, vaccineAvailabilityCount was reset to 0, so it
            # still ends up with the old avail_count (zero).
            previousEntry = siteOutputDict[key]
            siteOutputDict[key] = PrepModResult(key, case, name, \
                previousEntry.address, content_url, previousEntry.county, \
                previousEntry.city, previousEntry.avail_count + vaccineAvailabilityCount, \
                previousEntry.scraperTags.union(scraperTags))

          else:
            # Create new entry in siteOuptutDict, including address, county and city.
            # Create our own address because it is better than
            # the PrepMod address. Set the avail_count.

            street, city_state, zip = address.rsplit(', ', maxsplit=2)
            city, state = city_state.rsplit(' ', 1)

            # Make sure the address we're going to push to airtable has commas
            # separating street address, city, and state. The PrepMod addresses
            # leave a space between city and state, and that drives our testers
            # crazy. Also, some badly behaved sites neglect to specify state, or
            # spell it out rather than using the abbreviation, so fix that here.

            newAddress = street + ", " + city + ", " + 'WA' + " " + zip

            siteOutputDict[key] = PrepModResult(key, case, name, newAddress, \
                content_url, self.zipToCountyDict[zip], city, vaccineAvailabilityCount, \
                scraperTags)

        return siteOutputDict

    def generateURLForSite(self, sitename):
      nameElements = sitename.split(' ')
      siteNameForUrl = '+'.join(nameElements[0:2])

      parameterisedPrepmod ='https://prepmod.doh.wa.gov/appointment/en/clinic/search?location=&search_radius=All&q%5Bvenue_search_name_or_venue_name_i_cont%5D={}&clinic_date_eq%5Byear%5D=&clinic_date_eq%5Bmonth%5D=&clinic_date_eq%5Bday%5D=&q%5Bvaccinations_name_i_cont%5D=&commit=Search#search_results'
      return parameterisedPrepmod.format(siteNameForUrl)

    def generateKeyFromAddress(self, address):
      #Generates a key that should be unique, for a given site based off its address
      addressElements = address.split(" ")
      key = addressElements[0] + addressElements[len(addressElements)-1]

      return "autoprepmod" + key

    def checkIfSiteEntryHasAvailability(self, SitePElements):
      #Extract availabilityInt from string of format 'Availability     :     15'
      availabilityElement = [i for i in SitePElements if i.text.replace('\n','').startswith('Available Appointments:')]

      #This shouldn't happen
      if len(availabilityElement) < 1: #fix later
        logging.debug(f"Length of availabilityElement < 1")
        return 0
      if len(availabilityElement) > 1:
        raise ValueError('Two "Availability Elements" detected on PrepMod')

      return int(availabilityElement[0].text.split(":")[1].strip())

if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)
    scraper = PrepModSearch()
    siteOutputDict = scraper.MakeGetRequest()
    # logging.debug(siteOutputDict)
