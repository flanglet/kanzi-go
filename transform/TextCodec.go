/*
Copyright 2011-2021 Frederic Langlet
Licensed under the Apache License, Version 2.0 (the "License")
you may not use this file except in compliance with the License.
you may obtain a copy of the License at

                http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package transform

import (
	"errors"
	"fmt"

	kanzi "github.com/flanglet/kanzi-go"
)

const (
	// LF Line Feed symbol
	LF = byte(0x0A)
	// CR Carriage Return symbol
	CR = byte(0x0D)

	_TC_THRESHOLD1      = 128
	_TC_THRESHOLD2      = _TC_THRESHOLD1 * _TC_THRESHOLD1
	_TC_THRESHOLD3      = 32
	_TC_THRESHOLD4      = _TC_THRESHOLD3 * 128
	_TC_MAX_DICT_SIZE   = 1 << 19    // must be less than 1<<24
	_TC_MAX_WORD_LENGTH = 31         // must be less than 128
	_TC_LOG_HASHES_SIZE = 24         // 16 MB
	_TC_MAX_BLOCK_SIZE  = 1 << 30    // 1 GB
	_TC_ESCAPE_TOKEN1   = byte(0x0F) // dictionary word preceded by space symbol
	_TC_ESCAPE_TOKEN2   = byte(0x0E) // toggle upper/lower case of first word char
	_TC_MASK_NOT_TEXT   = 0x80
	_TC_MASK_DNA        = _TC_MASK_NOT_TEXT | 0x40
	_TC_MASK_BIN        = _TC_MASK_NOT_TEXT | 0x20
	_TC_MASK_BASE64     = _TC_MASK_NOT_TEXT | 0x10
	_TC_MASK_NUMERIC    = _TC_MASK_NOT_TEXT | 0x08
	_TC_MASK_FULL_ASCII = 0x04
	_TC_MASK_XML_HTML   = 0x02
	_TC_MASK_CRLF       = 0x01
	_TC_MASK_LENGTH     = 0x0007FFFF         // 19 bits
	_TC_HASH1           = int32(2146121005)  // 0x7FEB352D
	_TC_HASH2           = int32(-2073254261) // 0x846CA68B
)

type dictEntry struct {
	hash int32  // full word hash
	data int32  // packed word length (8 MSB) + index in dictionary (24 LSB)
	ptr  []byte // text data
}

// TextCodec is a simple one-pass text codec that replaces words with indexes.
// Uses a default (small) static dictionary. Generates a dynamic dictionary.
type TextCodec struct {
	delegate kanzi.ByteTransform
}

type textCodec1 struct {
	dictMap        []*dictEntry
	dictList       []dictEntry
	staticDictSize int
	dictSize       int
	logHashSize    uint
	hashMask       int32
	isCRLF         bool // EOL = CR+LF ?
	ctx            *map[string]interface{}
}

type textCodec2 struct {
	dictMap        []*dictEntry
	dictList       []dictEntry
	staticDictSize int
	dictSize       int
	logHashSize    uint
	hashMask       int32
	isCRLF         bool // EOL = CR+LF ?
	ctx            *map[string]interface{}
}

var (
	_TC_STATIC_DICTIONARY = [1024]dictEntry{}
	_TC_STATIC_DICT_WORDS = createDictionary(_TC_DICT_EN_1024, _TC_STATIC_DICTIONARY[:], 1024, 0)
	_TC_DELIMITER_CHARS   = initDelimiterChars()

	// Default dictionary
	// 1024 of the most common English words with at least 2 chars.
	_TC_DICT_EN_1024 = []byte(`TheBeAndOfInToWithItThatForYouHeHaveOnSaidSayAtButWeByHadTheyAsW
	ouldWhoOrCanMayDoThisWasIsMuchAnyFromNotSheWhatTheirWhichGetGive
	HasAreHimHerComeMyOurWereWillSomeBecauseThereThroughTellWhenWork
	ThemYetUpOwnOutIntoJustCouldOverOldThinkDayWayThanLikeOtherHowTh
	enItsPeopleTwoMoreTheseBeenNowWantFirstNewUseSeeTimeManManyThing
	MakeHereWellOnlyHisVeryAfterWithoutAnotherNoAllBelieveBeforeOffT
	houghSoAgainstWhileLastTooDownTodaySameBackTakeEachDifferentWher
	eBetweenThoseEvenSeenUnderAboutOneAlsoFactMustActuallyPreventExp
	ectContainConcernIfSchoolYearGoingCannotDueEverTowardGirlFirmGla
	ssGasKeepWorldStillWentShouldSpendStageDoctorMightJobGoContinueE
	veryoneNeverAnswerFewMeanDifferenceTendNeedLeaveTryNiceHoldSomet
	hingAskWarmLipCoverIssueHappenTurnLookSureDiscoverFightMadDirect
	ionAgreeSomeoneFailRespectNoticeChoiceBeginThreeSystemLevelFeelM
	eetCompanyBoxShowPlayLiveLetterEggNumberOpenProblemFatHandMeasur
	eQuestionCallRememberCertainPutNextChairStartRunRaiseGoalReallyH
	omeTeaCandidateMoneyBusinessYoungGoodCourtFindKnowKindHelpNightC
	hildLotYourUsEyeYesWordBitVanMonthHalfLowMillionHighOrganization
	RedGreenBlueWhiteBlackYourselfEightBothLittleHouseLetDespiteProv
	ideServiceHimselfFriendDescribeFatherDevelopmentAwayKillTripHour
	GameOftenPlantPlaceEndAmongSinceStandDesignParticularSuddenlyMem
	berPayLawBookSilenceAlmostIncludeAgainEitherToolFourOnceLeastExp
	lainIdentifyUntilSiteMinuteCoupleWeekMatterBringDetailInformatio
	nNothingAnythingEverythingAgoLeadSometimesUnderstandWhetherNatur
	eTogetherFollowParentStopIndeedDifficultPublicAlreadySpeakMainta
	inRemainHearAllowMediaOfficeBenefitDoorHugPersonLaterDuringWarHi
	storyArgueWithinSetArticleStationMorningWalkEventWinChooseBehavi
	orShootFireFoodTitleAroundAirTeacherGapSubjectEnoughProveAcrossA
	lthoughHeadFootSecondBoyMainLieAbleCivilTableLoveProcessOfferStu
	dentConsiderAppearStudyBuyNearlyHumanEvidenceTextMethodIncluding
	SendRealizeSenseBuildControlAudienceSeveralCutCollegeInterestSuc
	cessSpecialRiskExperienceBehindBetterResultTreatFiveRelationship
	AnimalImproveHairStayTopReducePerhapsLateWriterPickElseSignifica
	ntChanceHotelGeneralRockRequireAlongFitThemselvesReportCondition
	ReachTruthEffortDecideRateEducationForceGardenDrugLeaderVoiceQui
	teWholeSeemMindFinallySirReturnFreeStoryRespondPushAccordingBrot
	herLearnSonHopeDevelopFeelingReadCarryDiseaseRoadVariousBallCase
	OperationCloseVisitReceiveBuildingValueResearchFullModelJoinSeas
	onKnownDirectorPositionPlayerSportErrorRecordRowDataPaperTheoryS
	paceEveryFormSupportActionOfficialWhoseIdeaHappyHeartBestTeamPro
	jectHitBaseRepresentTownPullBusMapDryMomCatDadRoomSmileFieldImpa
	ctFundLargeDogHugePrepareEnvironmentalProduceHerselfTeachOilSuch
	SituationTieCostIndustrySkinStreetImageItselfPhonePriceWearMostS
	unSoonClearPracticePieceWaitRecentImportantProductLeftWallSeries
	NewsShareMovieKidNorSimplyWifeOntoCatchMyselfFineComputerSongAtt
	entionDrawFilmRepublicanSecurityScoreTestStockPositiveCauseCentu
	ryWindowMemoryExistListenStraightCultureBillionFormerDecisionEne
	rgyMoveSummerWonderRelateAvailableLineLikelyOutsideShotShortCoun
	tryRoleAreaSingleRuleDaughterMarketIndicatePresentLandCampaignMa
	terialPopulationEconomyMedicalHospitalChurchGroundThousandAuthor
	ityInsteadRecentlyFutureWrongInvolveLifeHeightIncreaseRightBankC
	ulturalCertainlyWestExecutiveBoardSeekLongOfficerStatementRestBa
	yDealWorkerResourceThrowForwardPolicyScienceEyesBedItemWeaponFil
	lPlanMilitaryGunHotHeatAddressColdFocusForeignTreatmentBloodUpon
	CourseThirdWatchAffectEarlyStoreThusSoundEverywhereBabyAdministr
	ationMouthPageEnterProbablyPointSeatNaturalRaceFarChallengePassA
	pplyMailUsuallyMixToughClearlyGrowFactorStateLocalGuyEastSaveSou
	thSceneMotherCareerQuicklyCentralFaceIceAboveBeyondPictureNetwor
	kManagementIndividualWomanSizeSpeedBusySeriousOccurAddReadySignC
	ollectionListApproachChargeQualityPressureVoteNotePartRealWebCur
	rentDetermineTrueSadWhateverBreakWorryCupParticularlyAmountAbili
	tyEatRecognizeSitCharacterSomebodyLossDegreeEffectAttackStaffMid
	dleTelevisionWhyLegalCapitalTradeElectionEverybodyDropMajorViewS
	tandardBillEmployeeDiscussionOpportunityAnalysisTenSuggestLawyer
	HusbandSectionBecomeSkillSisterStyleCrimeProgramCompareCapMissBa
	dSortTrainingEasyNearRegionStrategyPurposePerformTechnologyEcono
	micBudgetExampleCheckEnvironmentDoneDarkTermRatherLaughGuessCarL
	owerHangPastSocialForgetHundredRemoveManagerEnjoyExactlyDieFinal
	MaybeHealthFloorChangeAmericanPoorFunEstablishTrialSpringDinnerB
	igThankProtectAvoidImagineTonightStarArmFinishMusicOwnerCryArtPr
	ivateOthersSimplePopularReflectEspeciallySmallLightMessageStepKe
	yPeaceProgressMadeSideGreatFixInterviewManageNationalFishLoseCam
	eraDiscussEqualWeightPerformanceSevenWaterProductionPersonalCell
	PowerEveningColorInsideBarUnitLessAdultWideRangeMentionDeepEdgeS
	trongHardTroubleNecessarySafeCommonFearFamilySeaDreamConferenceR
	eplyPropertyMeetingAlwaysStuffAgencyDeathGrowthSellSoldierActHea
	vyWetBagMarriageDeadSingRiseDecadeWhomFigurePoliceBodyMachineCat
	egoryAheadFrontCareOrderRealityPartnerYardBeatViolenceTotalDefen
	seWriteConsumerCenterGroupThoughtModernTaskCoachReasonAgeFingerS
	pecificConnectionWishResponsePrettyMovementCardLogNumberSumTreeE
	ntireCitizenThroughoutPetSimilarVictimNewspaperThreatClassShakeS
	ourceAccountPainFallRichPossibleAcceptSolidTravelTalkSaidCreateN
	onePlentyPeriodDefineNormalRevealDrinkAuthorServeNameMomentAgent
	DocumentActivityAnywayAfraidTypeActiveTrainInterestingRadioDange
	rGenerationLeafCopyMatchClaimAnyoneSoftwarePartyDeviceCodeLangua
	geLinkHoweverConfirmCommentCityAnywhereSomewhereDebateDriveHighe
	rBeautifulOnlineFanPriorityTraditionalSixUnited`)

	_TC_BASE64_SYMBOLS  = []byte(`ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/`)
	_TC_NUMERIC_SYMBOLS = []byte(`0123456789+-*/=,.:; `)
	_TC_DNA_SYMBOLS     = []byte(`acgntuACGNTU"`) // either T or U and N for unknown
)

// Analyze the block and return an 8-bit status (see MASK flags constants)
// The goal is to detect test data amenable to pre-processing.
func computeStats(block []byte, freqs0 []int32, strict bool) byte {
	var freqs [256][256]int32
	freqs1 := freqs[0:256]
	count := len(block)
	end4 := count & -4
	prv := byte(0)

	// Unroll loop
	for i := 0; i < end4; i += 4 {
		cur0 := block[i]
		cur1 := block[i+1]
		cur2 := block[i+2]
		cur3 := block[i+3]
		freqs0[cur0]++
		freqs0[cur1]++
		freqs0[cur2]++
		freqs0[cur3]++
		freqs1[prv][cur0]++
		freqs1[cur0][cur1]++
		freqs1[cur1][cur2]++
		freqs1[cur2][cur3]++
		prv = cur3
	}

	for i := end4; i < count; i++ {
		cur := block[i]
		freqs0[cur]++
		freqs1[prv][cur]++
		prv = cur
	}

	nbTextChars := int(freqs0[CR]) + int(freqs0[LF])
	nbASCII := 0

	for i := 0; i < 128; i++ {
		if isText(byte(i)) == true {
			nbTextChars += int(freqs0[i])
		}

		nbASCII += int(freqs0[i])
	}

	// Not text (crude thresholds)
	if nbTextChars < (count>>1) || freqs0[32] < int32(count>>5) {
		return _TC_MASK_NOT_TEXT
	}

	if strict == true {
		if nbTextChars < (count>>2) || freqs0[0] >= int32(count/100) || (nbASCII/95) < (count/100) {
			return _TC_MASK_NOT_TEXT
		}
	}

	// Not text (crude threshold)
	var notText bool

	if strict == true {
		notText = ((nbTextChars < (count >> 2)) || (freqs0[0] >= int32(count/100)) || ((nbASCII / 95) < (count / 100)))
	} else {
		notText = (nbTextChars < (count >> 1)) || (freqs0[32] < int32(count>>5))
	}

	if notText == true {
		sum := int32(0)

		for i := 0; i < 12; i++ {
			sum += freqs0[_TC_DNA_SYMBOLS[i]]
		}

		if sum == int32(count) {
			return _TC_MASK_DNA
		}

		sum = 0

		for i := 0; i < 20; i++ {
			sum += freqs0[_TC_NUMERIC_SYMBOLS[i]]
		}

		if sum >= int32(count/100)*98 {
			return _TC_MASK_NUMERIC
		}

		sum = 0

		for i := 0; i < 64; i++ {
			sum += freqs0[_TC_BASE64_SYMBOLS[i]]
		}

		if sum == int32(count) {
			return _TC_MASK_BASE64
		}

		sum = 0

		for i := 0; i < 256; i++ {
			if freqs0[i] > 0 {
				sum++
			}
		}

		if sum == 255 {
			return _TC_MASK_BIN
		}

		return _TC_MASK_NOT_TEXT
	}

	nbBinChars := count - nbASCII
	res := byte(0)

	if nbBinChars > (count >> 2) {
		return _TC_MASK_NOT_TEXT
	}

	if nbBinChars == 0 {
		res |= _TC_MASK_FULL_ASCII
	}

	if nbBinChars <= count-count/10 {
		// Check if likely XML/HTML
		// Another crude test: check that the frequencies of < and > are similar
		// and 'high enough'. Also check it is worth to attempt replacing ampersand sequences.
		// Getting this flag wrong results in a very small compression speed degradation.
		f1 := freqs0['<']
		f2 := freqs0['>']
		f3 := freqs['&']['a'] + freqs['&']['g'] + freqs['&']['l'] + freqs['&']['q']
		minFreq := int32(count-nbBinChars) >> 9

		if minFreq < 2 {
			minFreq = 2
		}

		if (f1 >= minFreq) && (f2 >= minFreq) && (f3 > 0) {
			if f1 < f2 {
				if f1 >= f2-f2/100 {
					res |= _TC_MASK_XML_HTML
				}
			} else if f2 < f1 {
				if f2 >= f1-f1/100 {
					res |= _TC_MASK_XML_HTML
				}
			} else {
				res |= _TC_MASK_XML_HTML
			}
		}
	}

	if (freqs0[CR] != 0) && (freqs0[CR] == freqs0[LF]) {
		isCRLF := true

		for i := 0; i < 256; i++ {
			if (i != int(LF)) && (freqs1[CR][i] != 0) {
				isCRLF = false
				break
			}

			if (i != int(CR)) && (freqs1[i][LF] != 0) {
				isCRLF = false
				break
			}
		}

		if isCRLF == true {
			res |= _TC_MASK_CRLF
		}
	}

	return res
}

func sameWords(buf1, buf2 []byte) bool {
	for i := range buf1[1:] {
		if buf1[i] != buf2[i] {
			return false
		}
	}

	return true
}

func initDelimiterChars() []bool {
	var res [256]bool

	for i := range &res {
		if (i >= ' ') && (i <= '/') { // [ !"#$%&'()*+,-./]
			res[i] = true
			continue
		}

		if (i >= ':') && (i <= '?') { // [:;<=>?]
			res[i] = true
			continue
		}

		switch i {
		case '\n':
			res[i] = true
		case '\r':
			res[i] = true
		case '\t':
			res[i] = true
		case '_':
			res[i] = true
		case '|':
			res[i] = true
		case '{':
			res[i] = true
		case '}':
			res[i] = true
		case '[':
			res[i] = true
		case ']':
			res[i] = true
		default:
			res[i] = false
		}
	}

	return res[:]
}

// Create dictionary from array of words
func createDictionary(words []byte, dict []dictEntry, maxWords, startWord int) int {
	anchor := 0
	h := _TC_HASH1
	nbWords := startWord
	n := 0

	// Remove CR & LF symbols from list of English words
	for i := range words {
		if isText(words[i]) == false {
			continue
		}

		words[n] = words[i]
		n++
	}

	words = words[0:n]

	for i := 0; (i < len(words)) && (nbWords < maxWords); i++ {
		if isUpperCase(words[i]) {
			if i > anchor {
				dict[nbWords] = dictEntry{ptr: words[anchor:], hash: h, data: int32(((i - anchor) << 24) | nbWords)}
				nbWords++
				anchor = i
				h = _TC_HASH1
			}

			words[i] ^= 0x20
		}

		h = h*_TC_HASH1 ^ int32(words[i])*_TC_HASH2
	}

	if nbWords < maxWords {
		dict[nbWords] = dictEntry{ptr: words[anchor:], hash: h, data: int32(((len(words) - anchor) << 24) | nbWords)}
		nbWords++
	}

	return nbWords
}

func isText(val byte) bool {
	return isLowerCase(val) || isUpperCase(val)
}

func isLowerCase(val byte) bool {
	return (val >= 'a') && (val <= 'z')
}

func isUpperCase(val byte) bool {
	return (val >= 'A') && (val <= 'Z')
}

func isDelimiter(val byte) bool {
	return _TC_DELIMITER_CHARS[val]
}

// NewTextCodec creates a new instance of TextCodec
func NewTextCodec() (*TextCodec, error) {
	this := &TextCodec{}
	d, err := newTextCodec1()
	this.delegate = d
	return this, err
}

// NewTextCodecWithCtx creates a new instance of TextCodec using a
// configuration map as parameter.
func NewTextCodecWithCtx(ctx *map[string]interface{}) (*TextCodec, error) {
	this := &TextCodec{}

	var err error
	var d kanzi.ByteTransform

	if val, containsKey := (*ctx)["textcodec"]; containsKey {
		encodingType := val.(int)

		if encodingType == 2 {
			d, err = newTextCodec2WithCtx(ctx)
			this.delegate = d
		}
	}

	if this.delegate == nil && err == nil {
		d, err = newTextCodec1WithCtx(ctx)
		this.delegate = d
	}

	return this, err
}

// Forward applies the function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *TextCodec) Forward(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	if len(src) > _TC_MAX_BLOCK_SIZE {
		// Not a recoverable error: instead of silently fail the transform,
		// issue a fatal error.
		panic(fmt.Errorf("The max text transform block size is %d, got %d", _TC_MAX_BLOCK_SIZE, len(src)))
	}

	return this.delegate.Forward(src, dst)
}

// Inverse applies the reverse function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *TextCodec) Inverse(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	if len(src) > _TC_MAX_BLOCK_SIZE {
		// Not a recoverable error: instead of silently fail the transform,
		// issue a fatal error.
		panic(fmt.Errorf("The max text transform block size is %d, got %d", _TC_MAX_BLOCK_SIZE, len(src)))
	}

	return this.delegate.Inverse(src, dst)
}

// MaxEncodedLen returns the max size required for the encoding output buffer
func (this *TextCodec) MaxEncodedLen(srcLen int) int {
	return this.delegate.MaxEncodedLen(srcLen)
}

func newTextCodec1() (*textCodec1, error) {
	this := &textCodec1{}
	this.logHashSize = _TC_LOG_HASHES_SIZE
	this.dictSize = 1 << 13
	this.dictMap = make([]*dictEntry, 0)
	this.dictList = make([]dictEntry, 0)
	this.hashMask = int32(1<<this.logHashSize) - 1
	this.staticDictSize = _TC_STATIC_DICT_WORDS
	return this, nil
}

func newTextCodec1WithCtx(ctx *map[string]interface{}) (*textCodec1, error) {
	this := &textCodec1{}
	log := uint32(13)

	if val, containsKey := (*ctx)["blockSize"]; containsKey {
		blockSize := val.(uint)

		if blockSize >= 8 {
			log, _ = kanzi.Log2(uint32(blockSize / 8))

			if log > 26 {
				log = 26
			} else if log < 13 {
				log = 13
			}
		}
	}

	if val, containsKey := (*ctx)["extra"]; containsKey {
		if val.(bool) == true {
			log++
		}
	}

	this.logHashSize = uint(log)
	this.dictSize = 1 << 13
	this.dictMap = make([]*dictEntry, 0)
	this.dictList = make([]dictEntry, 0)
	this.hashMask = int32(1<<this.logHashSize) - 1
	this.staticDictSize = _TC_STATIC_DICT_WORDS
	this.ctx = ctx
	return this, nil
}

func (this *textCodec1) reset(count int) {
	if count >= 8 {
		// Select an appropriate initial dictionary size
		log, _ := kanzi.Log2(uint32(count / 8))

		if log > 22 {
			log = 22
		} else if log < 17 {
			log = 17
		}

		this.dictSize = 1 << (log - 4)
	}

	// Allocate lazily (only if text input detected)
	if len(this.dictMap) == 0 {
		this.dictMap = make([]*dictEntry, 1<<this.logHashSize)
	} else {
		for i := range this.dictMap {
			this.dictMap[i] = nil
		}
	}

	if len(this.dictList) == 0 {
		this.dictList = make([]dictEntry, this.dictSize)
		size := len(_TC_STATIC_DICTIONARY)

		if size >= this.dictSize {
			size = this.dictSize
		}

		copy(this.dictList, _TC_STATIC_DICTIONARY[0:size])

		// Add special entries at end of static dictionary
		this.dictList[_TC_STATIC_DICT_WORDS] = dictEntry{ptr: []byte{_TC_ESCAPE_TOKEN2}, hash: 0, data: int32((1 << 24) | (_TC_STATIC_DICT_WORDS))}
		this.dictList[_TC_STATIC_DICT_WORDS+1] = dictEntry{ptr: []byte{_TC_ESCAPE_TOKEN1}, hash: 0, data: int32((1 << 24) | (_TC_STATIC_DICT_WORDS + 1))}
		this.staticDictSize = _TC_STATIC_DICT_WORDS + 2
	}

	// Update map
	for i := 0; i < this.staticDictSize; i++ {
		e := this.dictList[i]
		this.dictMap[e.hash&this.hashMask] = &e
	}

	// Pre-allocate all dictionary entries
	for i := this.staticDictSize; i < this.dictSize; i++ {
		this.dictList[i] = dictEntry{ptr: nil, hash: 0, data: int32(i)}
	}
}

func (this *textCodec1) Forward(src, dst []byte) (uint, uint, error) {
	count := len(src)

	if n := this.MaxEncodedLen(count); len(dst) < n {
		return 0, 0, fmt.Errorf("Output buffer is too small - size: %d, required %d", len(dst), n)
	}

	if this.ctx != nil {
		if val, containsKey := (*this.ctx)["dataType"]; containsKey {
			dt := val.(kanzi.DataType)

			if dt != kanzi.DT_UNDEFINED && dt != kanzi.DT_TEXT {
				return 0, 0, fmt.Errorf("Input is not text, skip")
			}
		}
	}

	srcIdx := 0
	dstIdx := 0
	freqs0 := [256]int32{}
	mode := computeStats(src[0:count], freqs0[:], true)

	// Not text ?
	if mode&_TC_MASK_NOT_TEXT != 0 {
		if this.ctx != nil {
			switch mode {
			case _TC_MASK_NUMERIC:
				(*this.ctx)["dataType"] = kanzi.DT_NUMERIC
			case _TC_MASK_BASE64:
				(*this.ctx)["dataType"] = kanzi.DT_BASE64
			case _TC_MASK_BIN:
				(*this.ctx)["dataType"] = kanzi.DT_BIN
			case _TC_MASK_DNA:
				(*this.ctx)["dataType"] = kanzi.DT_DNA
			default:
			}
		}

		return uint(srcIdx), uint(dstIdx), errors.New("Input is not text, skip")
	}

	if this.ctx != nil {
		(*this.ctx)["dataType"] = kanzi.DT_TEXT
	}

	this.reset(count)
	srcEnd := count
	dstEnd := this.MaxEncodedLen(count)
	dstEnd4 := dstEnd - 4
	emitAnchor := 0 // never negative
	words := this.staticDictSize

	// DOS encoded end of line (CR+LF) ?
	this.isCRLF = mode&_TC_MASK_CRLF != 0
	dst[dstIdx] = mode
	dstIdx++
	var err error

	for srcIdx < srcEnd && src[srcIdx] == ' ' {
		dst[dstIdx] = ' '
		srcIdx++
		dstIdx++
		emitAnchor++
	}

	var delimAnchor int // previous delimiter

	if isText(src[srcIdx]) {
		delimAnchor = srcIdx - 1
	} else {
		delimAnchor = srcIdx
	}

	for srcIdx < srcEnd {
		cur := src[srcIdx]

		// Should be 'if isText(cur) ...', but compiler (1.11) issues slow code (bad inlining?)
		if isLowerCase(cur) || isUpperCase(cur) {
			srcIdx++
			continue
		}

		if (srcIdx > delimAnchor+2) && isDelimiter(cur) { // At least 2 letters
			length := int32(srcIdx - delimAnchor - 1)

			if length <= _TC_MAX_WORD_LENGTH {
				// Compute hashes
				// h1 -> hash of word chars
				// h2 -> hash of word chars with first char case flipped
				val := src[delimAnchor+1]
				h1 := _TC_HASH1
				h1 = h1*_TC_HASH1 ^ int32(val)*_TC_HASH2
				h2 := _TC_HASH1
				h2 = h2*_TC_HASH1 ^ (int32(val)^0x20)*_TC_HASH2

				for i := delimAnchor + 2; i < srcIdx; i++ {
					h := int32(src[i]) * _TC_HASH2
					h1 = h1*_TC_HASH1 ^ h
					h2 = h2*_TC_HASH1 ^ h
				}

				// Check word in dictionary
				var pe *dictEntry
				pe1 := this.dictMap[h1&this.hashMask]

				// Check for hash collisions
				if pe1 != nil && pe1.hash == h1 && pe1.data>>24 == length {
					pe = pe1
				}

				if pe == nil {
					if pe2 := this.dictMap[h2&this.hashMask]; pe2 != nil && pe2.hash == h2 && pe2.data>>24 == length {
						pe = pe2
					}
				}

				if pe != nil {
					if !sameWords(pe.ptr[1:length], src[delimAnchor+2:]) {
						pe = nil
					}
				}

				if pe == nil {
					// Word not found in the dictionary or hash collision.
					// Replace entry if not in static dictionary
					if ((length > 3) || (length == 3 && words < _TC_THRESHOLD2)) && (pe1 == nil) {
						pe = &this.dictList[words]

						if int(pe.data&_TC_MASK_LENGTH) >= this.staticDictSize {
							// Reuse old entry
							this.dictMap[pe.hash&this.hashMask] = nil
							pe.ptr = src[delimAnchor+1:]
							pe.hash = h1
							pe.data = (length << 24) | int32(words)
						}

						// Update hash map
						this.dictMap[h1&this.hashMask] = pe
						words++

						// Dictionary full ? Expand or reset index to end of static dictionary
						if words >= this.dictSize {
							if this.expandDictionary() == false {
								words = this.staticDictSize
							}
						}
					}
				} else {
					// Word found in the dictionary
					// Skip space if only delimiter between 2 word references
					if (emitAnchor != delimAnchor) || (src[delimAnchor] != ' ') {
						dIdx := this.emitSymbols(src[emitAnchor:delimAnchor+1], dst[dstIdx:dstEnd])

						if dIdx < 0 {
							err = errors.New("Text transform failed. Output buffer too small")
							break
						}

						dstIdx += dIdx
					}

					if dstIdx >= dstEnd4 {
						err = errors.New("Text transform failed. Output buffer too small")
						break
					}

					if pe == pe1 {
						dst[dstIdx] = _TC_ESCAPE_TOKEN1
					} else {
						dst[dstIdx] = _TC_ESCAPE_TOKEN2
					}

					dstIdx++
					dstIdx += emitWordIndex1(dst[dstIdx:dstIdx+3], int(pe.data&_TC_MASK_LENGTH))
					emitAnchor = delimAnchor + 1 + int(pe.data>>24)
				}
			}
		}

		// Reset delimiter position
		delimAnchor = srcIdx
		srcIdx++
	}

	if err == nil {
		// Emit last symbols
		dIdx := this.emitSymbols(src[emitAnchor:srcEnd], dst[dstIdx:dstEnd])

		if dIdx < 0 {
			err = errors.New("Text transform failed. Output buffer too small")
		} else {
			dstIdx += dIdx
		}
	}

	if err == nil && srcIdx != srcEnd {
		err = fmt.Errorf("Text transform failed. Source index: %d, expected: %d", srcIdx, srcEnd)
	}

	return uint(srcIdx), uint(dstIdx), err
}

func (this *textCodec1) expandDictionary() bool {
	if this.dictSize >= _TC_MAX_DICT_SIZE {
		return false
	}

	this.dictList = append(this.dictList, make([]dictEntry, this.dictSize)...)

	for i := this.dictSize; i < this.dictSize*2; i++ {
		this.dictList[i] = dictEntry{ptr: nil, hash: 0, data: int32(i)}
	}

	this.dictSize <<= 1
	return true
}

func (this *textCodec1) emitSymbols(src, dst []byte) int {
	dstIdx := 0
	dstEnd := len(dst)

	for _, cur := range src {
		if dstIdx >= dstEnd {
			return -1
		}

		switch cur {
		case _TC_ESCAPE_TOKEN1:
			fallthrough
		case _TC_ESCAPE_TOKEN2:
			// Emit special word
			dst[dstIdx] = _TC_ESCAPE_TOKEN1
			dstIdx++
			var idx int
			lenIdx := 2

			if cur == _TC_ESCAPE_TOKEN1 {
				idx = this.staticDictSize - 1
			} else {
				idx = this.staticDictSize - 2
			}

			if idx >= _TC_THRESHOLD2 {
				lenIdx = 3
			} else if idx < _TC_THRESHOLD1 {
				lenIdx = 1
			}

			if dstIdx+lenIdx >= dstEnd {
				return -1
			}

			dstIdx += emitWordIndex1(dst[dstIdx:dstIdx+lenIdx], idx)

		case CR:
			if this.isCRLF == false {
				dst[dstIdx] = cur
				dstIdx++
			}

		default:
			dst[dstIdx] = cur
			dstIdx++
		}
	}

	return dstIdx
}

func emitWordIndex1(dst []byte, val int) int {
	// Emit word index (varint 5 bits + 7 bits + 7 bits)
	if val < _TC_THRESHOLD1 {
		dst[0] = byte(val)
		return 1
	}

	if val < _TC_THRESHOLD2 {
		dst[0] = byte(0x80 | (val >> 7))
		dst[1] = byte(0x7F & val)
		return 2
	}

	dst[0] = byte(0xE0 | (val >> 14))
	dst[1] = byte(0x80 | (val >> 7))
	dst[2] = byte(0x7F & val)
	return 3
}

func (this *textCodec1) Inverse(src, dst []byte) (uint, uint, error) {
	srcIdx := 0
	dstIdx := 0
	this.reset(len(dst))
	srcEnd := len(src)
	dstEnd := len(dst)
	var delimAnchor int // previous delimiter

	if isText(src[srcIdx]) {
		delimAnchor = srcIdx - 1
	} else {
		delimAnchor = srcIdx
	}

	words := this.staticDictSize
	wordRun := false
	err := error(nil)
	this.isCRLF = src[srcIdx]&_TC_MASK_CRLF != 0
	srcIdx++

	for srcIdx < srcEnd && dstIdx < dstEnd {
		cur := src[srcIdx]

		if isText(cur) {
			dst[dstIdx] = cur
			srcIdx++
			dstIdx++
			continue
		}

		if (srcIdx > delimAnchor+3) && isDelimiter(cur) {
			length := int32(srcIdx - delimAnchor - 1) // length > 2

			if length <= _TC_MAX_WORD_LENGTH {
				h1 := _TC_HASH1

				for i := delimAnchor + 1; i < srcIdx; i++ {
					h1 = h1*_TC_HASH1 ^ int32(src[i])*_TC_HASH2
				}

				// Lookup word in dictionary
				var pe *dictEntry
				pe1 := this.dictMap[h1&this.hashMask]

				// Check for hash collisions
				if pe1 != nil && pe1.hash == h1 && pe1.data>>24 == length && sameWords(pe1.ptr[1:length], src[delimAnchor+2:]) {
					pe = pe1
				}

				if pe == nil {
					// Word not found in the dictionary or hash collision.
					// Replace entry if not in static dictionary
					if ((length > 3) || (words < _TC_THRESHOLD2)) && (pe1 == nil) {
						pe = &this.dictList[words]

						if int(pe.data&_TC_MASK_LENGTH) >= this.staticDictSize {
							// Reuse old entry
							this.dictMap[pe.hash&this.hashMask] = nil
							pe.ptr = src[delimAnchor+1:]
							pe.hash = h1
							pe.data = (length << 24) | int32(words)
						}

						this.dictMap[h1&this.hashMask] = pe
						words++

						// Dictionary full ? Expand or reset index to end of static dictionary
						if words >= this.dictSize {
							if this.expandDictionary() == false {
								words = this.staticDictSize
							}
						}
					}
				}
			}
		}

		srcIdx++

		if cur == _TC_ESCAPE_TOKEN1 || cur == _TC_ESCAPE_TOKEN2 {
			// Word in dictionary => read word index (varint 5 bits + 7 bits + 7 bits)
			idx := int(src[srcIdx])
			srcIdx++

			if idx >= 128 {
				idx &= 0x7F
				idx2 := int(src[srcIdx])
				srcIdx++

				if idx2 >= 0x80 {
					idx = ((idx & 0x1F) << 7) | (idx2 & 0x7F)
					idx2 = int(src[srcIdx]) & 0x7F
					srcIdx++
				}

				idx = (idx << 7) | idx2

				if idx >= this.dictSize {
					err = errors.New("Text transform failed. Invalid index")
					break
				}
			}

			pe := &this.dictList[idx]
			length := int(pe.data >> 24)

			// Sanity check
			if pe.ptr == nil || dstIdx+length >= dstEnd {
				err = errors.New("Text transform failed. Invalid input data")
				break
			}

			// Add space if only delimiter between 2 words (not an escaped delimiter)
			if length > 1 {
				if wordRun == true {
					dst[dstIdx] = ' '
					dstIdx++
				}

				// Regular word entry
				wordRun = true
				delimAnchor = srcIdx
			} else {
				// Escape entry
				wordRun = false
				delimAnchor = srcIdx - 1
			}

			// Emit word
			copy(dst[dstIdx:], pe.ptr[0:length])

			if cur == _TC_ESCAPE_TOKEN2 {
				// Flip case of first character
				dst[dstIdx] ^= 0x20
			}

			dstIdx += length
		} else {
			wordRun = false
			delimAnchor = srcIdx - 1

			if (this.isCRLF == true) && (cur == LF) {
				dst[dstIdx] = CR
				dstIdx++
			}

			dst[dstIdx] = cur
			dstIdx++
		}
	}

	if err == nil && srcIdx != srcEnd {
		err = fmt.Errorf("Text transform failed. Source index: %d, expected: %d", srcIdx, srcEnd)
	}

	return uint(srcIdx), uint(dstIdx), err
}

func (this textCodec1) MaxEncodedLen(srcLen int) int {
	// Limit to 1 x srcLength and let the caller deal with
	// a failure when the output is too small
	return srcLen
}

//nolint (remove unused warning)
func newTextCodec2() (*textCodec2, error) {
	this := &textCodec2{}
	this.logHashSize = _TC_LOG_HASHES_SIZE
	this.dictSize = 1 << 13
	this.dictMap = make([]*dictEntry, 0)
	this.dictList = make([]dictEntry, 0)
	this.hashMask = int32(1<<this.logHashSize) - 1
	this.staticDictSize = _TC_STATIC_DICT_WORDS
	return this, nil
}

func newTextCodec2WithCtx(ctx *map[string]interface{}) (*textCodec2, error) {
	this := &textCodec2{}
	log := uint32(13)

	if val, containsKey := (*ctx)["blockSize"]; containsKey {
		blockSize := val.(uint)

		if blockSize >= 32 {
			log, _ = kanzi.Log2(uint32(blockSize / 32))

			if log > 24 {
				log = 24
			} else if log < 13 {
				log = 13
			}
		}
	}

	if val, containsKey := (*ctx)["extra"]; containsKey {
		if val.(bool) == true {
			log++
		}
	}

	this.logHashSize = uint(log)
	this.dictSize = 1 << 13
	this.dictMap = make([]*dictEntry, 0)
	this.dictList = make([]dictEntry, 0)
	this.hashMask = int32(1<<this.logHashSize) - 1
	this.staticDictSize = _TC_STATIC_DICT_WORDS
	this.ctx = ctx
	return this, nil
}

func (this *textCodec2) reset(count int) {
	if count >= 8 {
		// Select an appropriate initial dictionary size
		log, _ := kanzi.Log2(uint32(count / 8))

		if log > 22 {
			log = 22
		} else if log < 17 {
			log = 17
		}

		this.dictSize = 1 << (log - 4)
	}

	// Allocate lazily (only if text input detected)
	if len(this.dictMap) == 0 {
		this.dictMap = make([]*dictEntry, 1<<this.logHashSize)
	} else {
		for i := range this.dictMap {
			this.dictMap[i] = nil
		}
	}

	if len(this.dictList) == 0 {
		this.dictList = make([]dictEntry, this.dictSize)
		size := len(_TC_STATIC_DICTIONARY)

		if size >= this.dictSize {
			size = this.dictSize
		}

		copy(this.dictList, _TC_STATIC_DICTIONARY[0:size])
	}

	// Update map
	for i := 0; i < this.staticDictSize; i++ {
		e := this.dictList[i]
		this.dictMap[e.hash&this.hashMask] = &e
	}

	// Pre-allocate all dictionary entries
	for i := this.staticDictSize; i < this.dictSize; i++ {
		this.dictList[i] = dictEntry{ptr: nil, hash: 0, data: int32(i)}
	}
}

func (this *textCodec2) Forward(src, dst []byte) (uint, uint, error) {
	count := len(src)

	if n := this.MaxEncodedLen(count); len(dst) < n {
		return 0, 0, fmt.Errorf("Output buffer is too small - size: %d, required %d", len(dst), n)
	}

	if this.ctx != nil {
		if val, containsKey := (*this.ctx)["dataType"]; containsKey {
			dt := val.(kanzi.DataType)

			if dt != kanzi.DT_UNDEFINED && dt != kanzi.DT_TEXT {
				return 0, 0, fmt.Errorf("Input is not text, skip")
			}
		}
	}

	srcIdx := 0
	dstIdx := 0
	freqs0 := [256]int32{}
	mode := computeStats(src[0:count], freqs0[:], false)

	// Not text ?
	if mode&_TC_MASK_NOT_TEXT != 0 {
		if this.ctx != nil {
			switch mode {
			case _TC_MASK_NUMERIC:
				(*this.ctx)["dataType"] = kanzi.DT_NUMERIC
			case _TC_MASK_BASE64:
				(*this.ctx)["dataType"] = kanzi.DT_BASE64
			case _TC_MASK_BIN:
				(*this.ctx)["dataType"] = kanzi.DT_BIN
			case _TC_MASK_DNA:
				(*this.ctx)["dataType"] = kanzi.DT_DNA
			default:
			}
		}

		return uint(srcIdx), uint(dstIdx), errors.New("Input is not text, skip")
	}

	if this.ctx != nil {
		(*this.ctx)["dataType"] = kanzi.DT_TEXT
	}

	this.reset(count)
	srcEnd := count
	dstEnd := this.MaxEncodedLen(count)
	dstEnd3 := dstEnd - 3
	emitAnchor := 0 // never negative
	words := this.staticDictSize

	// DOS encoded end of line (CR+LF) ?
	this.isCRLF = mode&_TC_MASK_CRLF != 0
	dst[dstIdx] = mode
	dstIdx++
	var err error

	for srcIdx < srcEnd && src[srcIdx] == ' ' {
		dst[dstIdx] = ' '
		srcIdx++
		dstIdx++
		emitAnchor++
	}

	var delimAnchor int // previous delimiter

	if isText(src[srcIdx]) {
		delimAnchor = srcIdx - 1
	} else {
		delimAnchor = srcIdx
	}

	for srcIdx < srcEnd {
		cur := src[srcIdx]

		// Should be 'if isText(cur) ...', but compiler (1.11) issues slow code (bad inlining?)
		if isLowerCase(cur) || isUpperCase(cur) {
			srcIdx++
			continue
		}

		if (srcIdx > delimAnchor+2) && isDelimiter(cur) { // At least 2 letters
			length := int32(srcIdx - delimAnchor - 1)

			if length <= _TC_MAX_WORD_LENGTH {
				// Compute hashes
				// h1 -> hash of word chars
				// h2 -> hash of word chars with first char case flipped
				val := src[delimAnchor+1]
				h1 := _TC_HASH1
				h1 = h1*_TC_HASH1 ^ int32(val)*_TC_HASH2
				h2 := _TC_HASH1
				h2 = h2*_TC_HASH1 ^ (int32(val)^0x20)*_TC_HASH2

				for i := delimAnchor + 2; i < srcIdx; i++ {
					h := int32(src[i]) * _TC_HASH2
					h1 = h1*_TC_HASH1 ^ h
					h2 = h2*_TC_HASH1 ^ h
				}

				// Check word in dictionary
				var pe *dictEntry
				pe1 := this.dictMap[h1&this.hashMask]
				var pe2 *dictEntry

				// Check for hash collisions
				if pe1 != nil && pe1.hash == h1 && pe1.data>>24 == length {
					pe = pe1
				}

				if pe == nil {
					if pe2 = this.dictMap[h2&this.hashMask]; pe2 != nil && pe2.hash == h2 && pe2.data>>24 == length {
						pe = pe2
					}
				}

				if pe != nil {
					if !sameWords(pe.ptr[1:length], src[delimAnchor+2:]) {
						pe = nil
					}
				}

				if pe == nil {
					// Word not found in the dictionary or hash collision.
					// Replace entry if not in static dictionary
					if ((length > 3) || (length == 3 && words < _TC_THRESHOLD2)) && (pe1 == nil) {
						pe = &this.dictList[words]

						if int(pe.data&_TC_MASK_LENGTH) >= this.staticDictSize {
							// Reuse old entry
							this.dictMap[pe.hash&this.hashMask] = nil
							pe.ptr = src[delimAnchor+1:]
							pe.hash = h1
							pe.data = (length << 24) | int32(words)
						}

						// Update hash map
						this.dictMap[h1&this.hashMask] = pe
						words++

						// Dictionary full ? Expand or reset index to end of static dictionary
						if words >= this.dictSize {
							if this.expandDictionary() == false {
								words = this.staticDictSize
							}
						}
					}
				} else {
					// Word found in the dictionary
					// Skip space if only delimiter between 2 word references
					if (emitAnchor != delimAnchor) || (src[delimAnchor] != ' ') {
						dIdx := this.emitSymbols(src[emitAnchor:delimAnchor+1], dst[dstIdx:dstEnd])

						if dIdx < 0 {
							err = errors.New("Text transform failed. Output buffer too small")
							break
						}

						dstIdx += dIdx
					}

					if dstIdx >= dstEnd3 {
						err = errors.New("Text transform failed. Output buffer too small")
						break
					}

					if pe == pe1 {
						dstIdx += emitWordIndex2(dst[dstIdx:dstIdx+3], int(pe.data&_TC_MASK_LENGTH), 0)
					} else {
						dstIdx += emitWordIndex2(dst[dstIdx:dstIdx+3], int(pe.data&_TC_MASK_LENGTH), 32)
					}

					emitAnchor = delimAnchor + 1 + int(pe.data>>24)
				}
			}
		}

		// Reset delimiter position
		delimAnchor = srcIdx
		srcIdx++
	}

	if err == nil {
		// Emit last symbols
		dIdx := this.emitSymbols(src[emitAnchor:srcEnd], dst[dstIdx:dstEnd])

		if dIdx < 0 {
			err = errors.New("Text transform failed. Output buffer too small")
		} else {
			dstIdx += dIdx
		}
	}

	if err == nil && srcIdx != srcEnd {
		err = fmt.Errorf("Text transform failed. Source index: %d, expected: %d", srcIdx, srcEnd)
	}

	return uint(srcIdx), uint(dstIdx), err
}

func (this *textCodec2) expandDictionary() bool {
	if this.dictSize >= _TC_MAX_DICT_SIZE {
		return false
	}

	this.dictList = append(this.dictList, make([]dictEntry, this.dictSize)...)

	for i := this.dictSize; i < this.dictSize*2; i++ {
		this.dictList[i] = dictEntry{ptr: nil, hash: 0, data: int32(i)}
	}

	this.dictSize <<= 1
	return true
}

func (this *textCodec2) emitSymbols(src, dst []byte) int {
	dstIdx := 0

	if 2*len(src) < len(dst) {
		for _, cur := range src {
			switch cur {
			case _TC_ESCAPE_TOKEN1:
				dst[dstIdx] = _TC_ESCAPE_TOKEN1
				dstIdx++
				dst[dstIdx] = _TC_ESCAPE_TOKEN1
				dstIdx++

			case CR:
				if this.isCRLF == false {
					dst[dstIdx] = cur
					dstIdx++
				}

			default:
				if cur >= 0x80 {
					dst[dstIdx] = _TC_ESCAPE_TOKEN1
					dstIdx++
				}

				dst[dstIdx] = cur
				dstIdx++
			}
		}
	} else {
		for _, cur := range src {
			switch cur {
			case _TC_ESCAPE_TOKEN1:
				if dstIdx+1 >= len(dst) {
					return -1
				}

				dst[dstIdx] = _TC_ESCAPE_TOKEN1
				dstIdx++
				dst[dstIdx] = _TC_ESCAPE_TOKEN1
				dstIdx++

			case CR:
				if this.isCRLF == false {
					if dstIdx >= len(dst) {
						return -1
					}

					dst[dstIdx] = cur
					dstIdx++
				}

			default:
				if cur >= 0x80 {
					if dstIdx >= len(dst) {
						return -1
					}

					dst[dstIdx] = _TC_ESCAPE_TOKEN1
					dstIdx++
				}

				if dstIdx >= len(dst) {
					return -1
				}

				dst[dstIdx] = cur
				dstIdx++
			}
		}
	}

	return dstIdx
}

func emitWordIndex2(dst []byte, val, mask int) int {
	// Emit word index (varint 5 bits + 7 bits + 7 bits)
	// 1st byte: 0x80 => word idx, 0x40 => more bytes, 0x20 => toggle case 1st symbol
	// 2nd byte: 0x80 => 1 more byte
	if val < _TC_THRESHOLD3 {
		dst[0] = byte(0x80 | mask | val)
		return 1
	}

	if val < _TC_THRESHOLD4 {
		// 5 + 7 => 2^12 = 32*128
		dst[0] = byte(0xC0 | mask | ((val >> 7) & 0x1F))
		dst[1] = byte(val & 0x7F)
		return 2
	}

	// 5 + 7 + 7 => 2^19
	dst[0] = byte(0xC0 | mask | ((val >> 14) & 0x1F))
	dst[1] = byte(0x80 | (val >> 7))
	dst[2] = byte(val & 0x7F)
	return 3
}

func (this *textCodec2) Inverse(src, dst []byte) (uint, uint, error) {
	srcIdx := 0
	dstIdx := 0
	this.reset(len(dst))
	srcEnd := len(src)
	dstEnd := len(dst)
	var delimAnchor int // previous delimiter

	if isText(src[srcIdx]) {
		delimAnchor = srcIdx - 1
	} else {
		delimAnchor = srcIdx
	}

	words := this.staticDictSize
	wordRun := false
	err := error(nil)
	this.isCRLF = src[srcIdx]&_TC_MASK_CRLF != 0
	srcIdx++

	for srcIdx < srcEnd && dstIdx < dstEnd {
		cur := src[srcIdx]

		if isText(cur) {
			dst[dstIdx] = cur
			srcIdx++
			dstIdx++
			continue
		}

		if (srcIdx > delimAnchor+3) && isDelimiter(cur) {
			length := int32(srcIdx - delimAnchor - 1) // length > 2

			if length <= _TC_MAX_WORD_LENGTH {
				h1 := _TC_HASH1

				for i := delimAnchor + 1; i < srcIdx; i++ {
					h1 = h1*_TC_HASH1 ^ int32(src[i])*_TC_HASH2
				}

				// Lookup word in dictionary
				var pe *dictEntry
				pe1 := this.dictMap[h1&this.hashMask]

				// Check for hash collisions
				if pe1 != nil && pe1.hash == h1 && pe1.data>>24 == length && sameWords(pe1.ptr[1:length], src[delimAnchor+2:]) {
					pe = pe1
				}

				if pe == nil {
					// Word not found in the dictionary or hash collision.
					// Replace entry if not in static dictionary
					if ((length > 3) || (words < _TC_THRESHOLD2)) && (pe1 == nil) {
						pe = &this.dictList[words]

						if int(pe.data&_TC_MASK_LENGTH) >= this.staticDictSize {
							// Reuse old entry
							this.dictMap[pe.hash&this.hashMask] = nil
							pe.ptr = src[delimAnchor+1:]
							pe.hash = h1
							pe.data = (length << 24) | int32(words)
						}

						this.dictMap[h1&this.hashMask] = pe
						words++

						// Dictionary full ? Expand or reset index to end of static dictionary
						if words >= this.dictSize {
							if this.expandDictionary() == false {
								words = this.staticDictSize
							}
						}
					}
				}
			}
		}

		srcIdx++

		if cur >= 128 {
			// Word in dictionary => read word index (varint 5 bits + 7 bits + 7 bits)
			idx := int(cur & 0x1F)

			if cur&0x40 != 0 {
				idx2 := int(src[srcIdx])
				srcIdx++

				if idx2 >= 128 {
					idx = (idx << 7) | (idx2 & 0x7F)
					idx2 = int(src[srcIdx]) & 0x7F
					srcIdx++
				}

				idx = (idx << 7) | idx2

				if idx >= this.dictSize {
					err = errors.New("Text transform failed. Invalid index")
					break
				}
			}

			pe := &this.dictList[idx]
			length := int(pe.data >> 24)

			// Sanity check
			if pe.ptr == nil || dstIdx+length >= dstEnd {
				err = errors.New("Text transform failed. Invalid input data")
				break
			}

			// Add space if only delimiter between 2 words (not an escaped delimiter)
			if length > 1 {
				if wordRun == true {
					dst[dstIdx] = ' '
					dstIdx++
				}

				// Regular word entry
				wordRun = true
				delimAnchor = srcIdx
			} else {
				// Escape entry
				wordRun = false
				delimAnchor = srcIdx - 1
			}

			// Emit word
			copy(dst[dstIdx:], pe.ptr[0:length])

			// Flip case of first character
			dst[dstIdx] ^= (cur & 0x20)
			dstIdx += length
		} else {
			if cur == _TC_ESCAPE_TOKEN1 {
				dst[dstIdx] = src[srcIdx]
				srcIdx++
				dstIdx++
			} else {
				if (this.isCRLF == true) && (cur == LF) {
					dst[dstIdx] = CR
					dstIdx++
				}

				dst[dstIdx] = cur
				dstIdx++
			}

			wordRun = false
			delimAnchor = srcIdx - 1
		}
	}

	if err == nil && srcIdx != srcEnd {
		err = fmt.Errorf("Text transform failed. Source index: %d, expected: %d", srcIdx, srcEnd)
	}

	return uint(srcIdx), uint(dstIdx), err
}

func (this textCodec2) MaxEncodedLen(srcLen int) int {
	// Limit to 1 x srcLength and let the caller deal with
	// a failure when the output is too small
	return srcLen
}
