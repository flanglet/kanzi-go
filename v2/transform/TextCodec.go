/*
Copyright 2011-2025 Frederic Langlet
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

	kanzi "github.com/flanglet/kanzi-go/v2"
	internal "github.com/flanglet/kanzi-go/v2/internal"
)

const (
	// LF Line Feed symbol
	LF = byte(0x0A)
	// CR Carriage Return symbol
	CR = byte(0x0D)

	_TC_THRESHOLD1      = 128
	_TC_THRESHOLD2      = _TC_THRESHOLD1 * _TC_THRESHOLD1
	_TC_THRESHOLD3      = 64
	_TC_THRESHOLD4      = _TC_THRESHOLD3 * 128
	_TC_MAX_DICT_SIZE   = 1 << 19 // must be less than 1<<24
	_TC_MAX_WORD_LENGTH = 31      // must be less than 128
	_TC_LOG_HASHES_SIZE = 24      // 16 MB
	_TC_MIN_BLOCK_SIZE  = 1024
	_TC_MAX_BLOCK_SIZE  = 1 << 30    // 1 GB
	_TC_ESCAPE_TOKEN1   = byte(0x0F) // dictionary word preceded by space symbol
	_TC_ESCAPE_TOKEN2   = byte(0x0E) // toggle upper/lower case of first word char
	_TC_MASK_FLIP_CASE  = 0x80
	_TC_MASK_NOT_TEXT   = 0x80
	_TC_MASK_CRLF       = 0x40
	_TC_MASK_XML_HTML   = 0x20
	_TC_MASK_DT         = 0x0F
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
	ctx            *map[string]any
}

type textCodec2 struct {
	dictMap        []*dictEntry
	dictList       []dictEntry
	staticDictSize int
	dictSize       int
	logHashSize    uint
	hashMask       int32
	bsVersion      uint
	isCRLF         bool // EOL = CR+LF ?
	ctx            *map[string]any
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
)

// Analyze the block and return an 8-bit status (see MASK flags constants)
// The goal is to detect text data amenable to pre-processing.
func computeTextStats(block []byte, freqs0 []int, strict bool) byte {
	if strict == false && internal.GetMagicType(block) != internal.NO_MAGIC {
		// This is going to fail if the block is not the first of the file.
		// But this is a cheap test, good enough for fast mode.
		return _TC_MASK_NOT_TEXT
	}

	freqs1 := make([][256]int, 256)
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

	// Not text (crude threshold)
	nbBinChars := count - nbASCII
	notText := false

	if nbBinChars > (count >> 2) {
		notText = true
	} else {
		notText = nbTextChars < (count / 4)

		if strict == true {
			notText = notText || ((freqs0[0] >= (count / 100)) || ((nbASCII / 95) < (count / 100)))
		} else {
			notText = notText || (freqs0[32] < (count / 50))
		}
	}

	res := byte(0)

	if notText == true {
		return res | detectTextType(freqs0, freqs1[:], count)
	}

	if nbBinChars <= count-count/10 {
		// Check if likely XML/HTML
		// Another crude test: check that the frequencies of < and > are similar
		// and 'high enough'. Also check it is worth to attempt replacing ampersand sequences.
		// Getting this flag wrong results in a very small compression speed degradation.
		f1 := freqs0['<']
		f2 := freqs0['>']
		f3 := freqs1['&']['a'] + freqs1['&']['g'] + freqs1['&']['l'] + freqs1['&']['q']
		minFreq := (count - nbBinChars) >> 9

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

func detectTextType(freqs0 []int, freqs [][256]int, count int) byte {
	if dt := internal.DetectSimpleType(count, freqs0); dt != internal.DT_UNDEFINED {
		return _TC_MASK_NOT_TEXT | byte(dt)
	}

	// Valid UTF-8 sequences
	// See Unicode 16 Standard - UTF-8 Table 3.7
	// U+0000..U+007F          00..7F
	// U+0080..U+07FF          C2..DF 80..BF
	// U+0800..U+0FFF          E0 A0..BF 80..BF
	// U+1000..U+CFFF          E1..EC 80..BF 80..BF
	// U+D000..U+D7FF          ED 80..9F 80..BF 80..BF
	// U+E000..U+FFFF          EE..EF 80..BF 80..BF
	// U+10000..U+3FFFF        F0 90..BF 80..BF 80..BF
	// U+40000..U+FFFFF        F1..F3 80..BF 80..BF 80..BF
	// U+100000..U+10FFFF      F4 80..8F 80..BF 80..BF

	// Check rules for 1 byte
	sum := freqs0[0xC0] + freqs0[0xC1]

	for _, f := range freqs0[0xF5:] {
		sum += f
	}

	if sum != 0 {
		return _TC_MASK_NOT_TEXT
	}

	sum2 := 0

	// Check rules for first 2 bytes
	for i := 0; i < 256; i++ {
		// Exclude < 0xE0A0 || > 0xE0BF
		if i < 0xA0 || i > 0xBF {
			sum += freqs[0xE0][i]
		}

		// Exclude < 0xED80 || > 0xEDE9F
		if i < 0x80 || i > 0x9F {
			sum += freqs[0xED][i]
		}

		// Exclude < 0xF090 || > 0xF0BF
		if i < 0x90 || i > 0xBF {
			sum += freqs[0xF0][i]
		}

		// Exclude < 0xF480 || > 0xF48F
		if i < 0x80 || i > 0x8F {
			sum += freqs[0xF4][i]
		}

		if i < 0x80 || i > 0xBF {
			// Exclude < 0x??80 || > 0x??BF with ?? in [C2..DF]
			for j := 0xC2; j <= 0xDF; j++ {
				sum += freqs[j][i]
			}

			// Exclude < 0x??80 || > 0x??BF with ?? in [E1..EC]
			for j := 0xE1; j <= 0xEC; j++ {
				sum += freqs[j][i]
			}

			// Exclude < 0x??80 || > 0x??BF with ?? in [F1..F3]
			sum += freqs[0xF1][i]
			sum += freqs[0xF2][i]
			sum += freqs[0xF3][i]

			// Exclude < 0xEE80 || > 0xEEBF
			sum += freqs[0xEE][i]

			// Exclude < 0xEF80 || > 0xEFBF
			sum += freqs[0xEF][i]
		} else {
			// Count non-primary bytes
			sum2 += freqs0[i]
		}

		if sum != 0 {
			return _TC_MASK_NOT_TEXT
		}
	}

	// Ad-hoc threshold
	if sum2 >= count/8 {
		return _TC_MASK_NOT_TEXT | byte(internal.DT_UTF8)
	} else {
		return _TC_MASK_NOT_TEXT
	}
}

func sameWords(buf1, buf2 []byte) bool {
	for i := range buf1 {
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
	return isLowerCase(val | 0x20)
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
func NewTextCodecWithCtx(ctx *map[string]any) (*TextCodec, error) {
	this := &TextCodec{}

	var err error
	var d kanzi.ByteTransform

	if ctx != nil {
		if val, hasKey := (*ctx)["textcodec"]; hasKey {
			encodingType := val.(int)

			if encodingType == 2 {
				d, err = newTextCodec2WithCtx(ctx)
				this.delegate = d
			}
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

	if len(src) < _TC_MIN_BLOCK_SIZE {
		return 0, 0, fmt.Errorf("The min text transform block size is %d, got %d", _TC_MIN_BLOCK_SIZE, len(src))
	}

	if len(src) > _TC_MAX_BLOCK_SIZE {
		return 0, 0, fmt.Errorf("The max text transform block size is %d, got %d", _TC_MAX_BLOCK_SIZE, len(src))
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
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

	// ! no min test
	if len(src) > _TC_MAX_BLOCK_SIZE {
		return 0, 0, fmt.Errorf("The max text transform block size is %d, got %d", _TC_MAX_BLOCK_SIZE, len(src))
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
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

func newTextCodec1WithCtx(ctx *map[string]any) (*textCodec1, error) {
	this := &textCodec1{}
	log := uint32(13)

	if ctx != nil {
		if val, hasKey := (*ctx)["blockSize"]; hasKey {
			blockSize := val.(uint)

			if blockSize >= 8 {
				log, _ = internal.Log2(uint32(blockSize / 8))
				log = min(log, 26)
				log = max(log, 13)
			}
		}

		if val, hasKey := (*ctx)["entropy"]; hasKey {
			if val.(string) == "TPAQX" {
				log++
			}
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
	if count >= 1024 {
		// Select an appropriate initial dictionary size
		log, _ := internal.Log2(uint32(count / 128))
		log = min(log, 18)
		log = max(log, 13)
		this.dictSize = 1 << log
	}

	// Allocate lazily (only if text input detected)
	if len(this.dictMap) < 1<<this.logHashSize {
		this.dictMap = make([]*dictEntry, 1<<this.logHashSize)
	} else {
		for i := range this.dictMap {
			this.dictMap[i] = nil
		}
	}

	if len(this.dictList) < this.dictSize {
		this.dictList = make([]dictEntry, this.dictSize)
		size := min(len(_TC_STATIC_DICTIONARY), this.dictSize)
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
		if val, hasKey := (*this.ctx)["dataType"]; hasKey {
			dt := val.(internal.DataType)

			// Filter out most types. Still check binaries which may contain significant parts of text
			if dt != internal.DT_UNDEFINED && dt != internal.DT_TEXT && dt != internal.DT_BIN {
				return 0, 0, fmt.Errorf("Input is not text, skip")
			}
		}
	}

	freqs0 := [256]int{}
	mode := computeTextStats(src[0:count], freqs0[:], true)

	// Not text ?
	if mode&_TC_MASK_NOT_TEXT != 0 {
		if this.ctx != nil {
			(*this.ctx)["dataType"] = internal.DataType(mode & _TC_MASK_DT)
		}

		return 0, 0, errors.New("Input is not text, skip")
	}

	if this.ctx != nil {
		(*this.ctx)["dataType"] = internal.DT_TEXT
	}

	this.reset(count)
	srcEnd := count
	dstEnd := this.MaxEncodedLen(count)
	dstEnd4 := dstEnd - 4
	emitAnchor := 0 // never negative
	words := this.staticDictSize

	// mode 0xx00000 : 5 bits available
	// DOS encoded end of line (CR+LF) ?
	this.isCRLF = mode&_TC_MASK_CRLF != 0
	dst[0] = mode
	dstIdx := 1
	srcIdx := 0

	for srcIdx < srcEnd && src[srcIdx] == ' ' {
		dst[dstIdx] = ' '
		srcIdx++
		dstIdx++
		emitAnchor++
	}

	var err error
	var delimAnchor int // previous delimiter

	if isText(src[srcIdx]) {
		delimAnchor = srcIdx - 1
	} else {
		delimAnchor = srcIdx
	}

	for srcIdx < srcEnd {
		if isText(src[srcIdx]) {
			srcIdx++
			continue
		}

		if (srcIdx > delimAnchor+2) && isDelimiter(src[srcIdx]) { // At least 2 letters
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

				if pe1 != nil && pe1.hash == h1 && pe1.data>>24 == length {
					pe = pe1
				} else if pe2 := this.dictMap[h2&this.hashMask]; pe2 != nil && pe2.hash == h2 && pe2.data>>24 == length {
					pe = pe2
				}

				// Check for hash collisions
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
						dstIdx += this.emitSymbols(src[emitAnchor:delimAnchor+1], dst[dstIdx:dstEnd])
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
		dstIdx += this.emitSymbols(src[emitAnchor:srcEnd], dst[dstIdx:dstEnd])

		if dstIdx > dstEnd {
			err = errors.New("Text transform failed. Output buffer too small")
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
			return dstEnd + 1
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
				return dstEnd + 1
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
	this.reset(len(dst))
	srcEnd := len(src)
	dstEnd := len(dst)
	words := this.staticDictSize
	wordRun := false
	err := error(nil)
	this.isCRLF = src[0]&_TC_MASK_CRLF != 0
	srcIdx := 1
	dstIdx := 0
	delimAnchor := 0 // previous delimiter

	if isText(src[srcIdx]) {
		delimAnchor = srcIdx - 1
	} else {
		delimAnchor = srcIdx
	}

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
				h1 = h1*_TC_HASH1 ^ int32(src[delimAnchor+1])*_TC_HASH2
				h1 = h1*_TC_HASH1 ^ int32(src[delimAnchor+2])*_TC_HASH2

				for i := delimAnchor + 3; i < srcIdx; i++ {
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
					idx2 = int(src[srcIdx])
					srcIdx++
				}

				idx = (idx << 7) | idx2

				if idx >= this.dictSize {
					err = errors.New("Text transform failed. Invalid index")
					break
				}
			}

			pe := &this.dictList[idx]
			length := int(pe.data>>24) & 0xFF

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

			// Sanity check
			if pe.ptr == nil || dstIdx+length >= dstEnd {
				err = errors.New("Text transform failed. Invalid input data")
				break
			}

			// Emit word
			copy(dst[dstIdx:], pe.ptr[0:length])

			// Flip case of first character ?
			if cur == _TC_ESCAPE_TOKEN2 {
				dst[dstIdx] ^= 0x20
			}

			dstIdx += length
		} else {
			wordRun = false
			delimAnchor = srcIdx - 1

			if (this.isCRLF == true) && (cur == LF) {
				dst[dstIdx] = CR
				dstIdx++

				if dstIdx >= dstEnd {
					err = errors.New("Text transform failed. Invalid input data")
					break
				}
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

func (this *textCodec1) MaxEncodedLen(srcLen int) int {
	// Limit to 1 x srcLength and let the caller deal with
	// a failure when the output is too small
	return srcLen
}

// nolint (remove unused warning)
func newTextCodec2() (*textCodec2, error) {
	this := &textCodec2{}
	this.logHashSize = _TC_LOG_HASHES_SIZE
	this.dictSize = 1 << 13
	this.dictMap = make([]*dictEntry, 0)
	this.dictList = make([]dictEntry, 0)
	this.hashMask = int32(1<<this.logHashSize) - 1
	this.staticDictSize = _TC_STATIC_DICT_WORDS
	this.bsVersion = 6
	return this, nil
}

func newTextCodec2WithCtx(ctx *map[string]any) (*textCodec2, error) {
	this := &textCodec2{}
	log := uint32(13)
	version := uint(6)

	if ctx != nil {
		if val, hasKey := (*ctx)["blockSize"]; hasKey {
			blockSize := val.(uint)

			if blockSize >= 32 {
				log, _ = internal.Log2(uint32(blockSize / 32))
				log = min(log, 24)
				log = max(log, 13)
			}
		}

		if val, hasKey := (*ctx)["entropy"]; hasKey {
			if val.(string) == "TPAQX" {
				log++
			}
		}

		if val, hasKey := (*ctx)["bsVersion"]; hasKey {
			version = val.(uint)
		}
	}

	this.logHashSize = uint(log)
	this.dictSize = 1 << 13
	this.dictMap = make([]*dictEntry, 0)
	this.dictList = make([]dictEntry, 0)
	this.hashMask = int32(1<<this.logHashSize) - 1
	this.staticDictSize = _TC_STATIC_DICT_WORDS
	this.ctx = ctx
	this.bsVersion = version
	return this, nil
}

func (this *textCodec2) reset(count int) {
	if count >= 1024 {
		// Select an appropriate initial dictionary size
		log, _ := internal.Log2(uint32(count / 128))
		log = min(log, 18)
		log = max(log, 13)
		this.dictSize = 1 << log
	}

	// Allocate lazily (only if text input detected)
	if len(this.dictMap) < 1<<this.logHashSize {
		this.dictMap = make([]*dictEntry, 1<<this.logHashSize)
	} else {
		for i := range this.dictMap {
			this.dictMap[i] = nil
		}
	}

	if len(this.dictList) < this.dictSize {
		this.dictList = make([]dictEntry, this.dictSize)
		size := min(len(_TC_STATIC_DICTIONARY), this.dictSize)
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
		if val, hasKey := (*this.ctx)["dataType"]; hasKey {
			dt := val.(internal.DataType)

			// Filter out most types. Still check binaries which may contain significant parts of text
			if dt != internal.DT_UNDEFINED && dt != internal.DT_TEXT && dt != internal.DT_BIN {
				return 0, 0, fmt.Errorf("Input is not text, skip")
			}
		}
	}

	freqs0 := [256]int{}
	mode := computeTextStats(src[0:count], freqs0[:], false)

	// Not text ?
	if mode&_TC_MASK_NOT_TEXT != 0 {
		if this.ctx != nil {
			(*this.ctx)["dataType"] = internal.DataType(mode & _TC_MASK_DT)
		}

		return uint(0), uint(0), errors.New("Input is not text, skip")
	}

	if this.ctx != nil {
		(*this.ctx)["dataType"] = internal.DT_TEXT
	}

	this.reset(count)
	srcEnd := count
	dstEnd := this.MaxEncodedLen(count)
	dstEnd3 := dstEnd - 3
	emitAnchor := 0 // never negative
	words := this.staticDictSize

	// mode 0xx00000 : 5 bits available
	// DOS encoded end of line (CR+LF) ?
	this.isCRLF = mode&_TC_MASK_CRLF != 0
	dst[0] = mode
	srcIdx := 0
	dstIdx := 1

	for srcIdx < srcEnd && src[srcIdx] == ' ' {
		dst[dstIdx] = ' '
		srcIdx++
		dstIdx++
		emitAnchor++
	}

	var err error
	delimAnchor := srcIdx // previous delimiter

	if isText(src[srcIdx]) {
		delimAnchor = srcIdx - 1
	}

	for srcIdx < srcEnd {
		if isText(src[srcIdx]) {
			srcIdx++
			continue
		}

		if (srcIdx > delimAnchor+2) && isDelimiter(src[srcIdx]) { // At least 2 letters
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

				if pe1 != nil && pe1.hash == h1 && pe1.data>>24 == length {
					pe = pe1
				} else {
					if pe2 := this.dictMap[h2&this.hashMask]; pe2 != nil && pe2.hash == h2 && pe2.data>>24 == length {
						pe = pe2
					}
				}

				// Check for hash collisions
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
						dstIdx += this.emitSymbols(src[emitAnchor:delimAnchor+1], dst[dstIdx:dstEnd])
					}

					if dstIdx >= dstEnd3 {
						err = errors.New("Text transform failed. Output buffer too small")
						break
					}

					if pe != pe1 {
						dst[dstIdx] = _TC_MASK_FLIP_CASE
						dstIdx++
					}

					dstIdx += emitWordIndex2(dst[dstIdx:dstIdx+3], int(pe.data&_TC_MASK_LENGTH))
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
		dstIdx += this.emitSymbols(src[emitAnchor:srcEnd], dst[dstIdx:dstEnd])

		if dstIdx > dstEnd {
			err = errors.New("Text transform failed. Output buffer too small")
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
					return len(dst) + 1
				}

				dst[dstIdx] = _TC_ESCAPE_TOKEN1
				dstIdx++
				dst[dstIdx] = _TC_ESCAPE_TOKEN1
				dstIdx++

			case CR:
				if this.isCRLF == false {
					if dstIdx >= len(dst) {
						return len(dst) + 1
					}

					dst[dstIdx] = cur
					dstIdx++
				}

			default:
				if cur >= 0x80 {
					if dstIdx >= len(dst) {
						return len(dst) + 1
					}

					dst[dstIdx] = _TC_ESCAPE_TOKEN1
					dstIdx++
				}

				if dstIdx >= len(dst) {
					return len(dst) + 1
				}

				dst[dstIdx] = cur
				dstIdx++
			}
		}
	}

	return dstIdx
}

func emitWordIndex2(dst []byte, wIdx int) int {
	// 0x80 is reserved to first symbol case flip
	wIdx++

	if wIdx >= _TC_THRESHOLD3 {
		if wIdx >= _TC_THRESHOLD4 {
			// 3 byte index (1111xxxx xxxxxxxx xxxxxxxx)
			dst[0] = byte(0xF0 | (wIdx >> 16))
			dst[1] = byte(wIdx >> 8)
			dst[2] = byte(wIdx)
			return 3
		}

		// 2 byte index (110xxxxx xxxxxxxx)
		dst[0] = byte(0xC0 | (wIdx >> 8))
		dst[1] = byte(wIdx)
		return 2
	}

	// 1 byte index (10xxxxxx) with 0x80 excluded
	dst[0] = byte(0x80 | wIdx)
	return 1
}

func (this *textCodec2) Inverse(src, dst []byte) (uint, uint, error) {
	this.reset(len(dst))
	words := this.staticDictSize
	wordRun := false
	var err error
	this.isCRLF = src[0]&_TC_MASK_CRLF != 0
	srcIdx := 1
	dstIdx := 0
	srcEnd := len(src)
	dstEnd := len(dst)
	oldEncoding := this.bsVersion < 6
	delimAnchor := srcIdx // previous delimiter

	if isText(src[srcIdx]) {
		delimAnchor = srcIdx - 1
	}

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
				h1 = h1*_TC_HASH1 ^ int32(src[delimAnchor+1])*_TC_HASH2
				h1 = h1*_TC_HASH1 ^ int32(src[delimAnchor+2])*_TC_HASH2

				for i := delimAnchor + 3; i < srcIdx; i++ {
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
		flipMask := byte(0)

		if cur >= 128 {
			// Word in dictionary => read word index (varint 5 bits + 7 bits + 7 bits)
			var idx int

			if oldEncoding == true {
				idx = int(cur & 0x1F)
				flipMask = cur & 0x20

				if cur&0x40 != 0 {
					idx2 := int(src[srcIdx])
					srcIdx++

					if idx2 >= 128 {
						idx = (idx << 7) | (idx2 & 0x7F)
						idx2 = int(src[srcIdx])
						srcIdx++
					}

					idx = (idx << 7) | idx2

					// Sanity check
					if idx >= this.dictSize {
						err = errors.New("Text transform failed. Invalid index")
						break
					}
				}
			} else {
				if cur == _TC_MASK_FLIP_CASE {
					// Flip first char case
					flipMask = 0x20
					cur = src[srcIdx]
					srcIdx++
				}

				// Read word index
				// 10xxxxxx => 1 byte
				// 110xxxxx => 2 bytes
				// 1111xxxx => 3 bytes
				idx = int(cur) & 0x7F

				if idx >= 64 {
					if idx >= 112 {
						idx = ((idx & 0x0F) << 16) | (int(src[srcIdx]) << 8) | int(src[srcIdx+1])
						srcIdx += 2
					} else {
						idx = ((idx & 0x1F) << 8) | int(src[srcIdx])
						srcIdx++
					}

					// Sanity check before adjusting index
					if idx > this.dictSize {
						err = errors.New("Text transform failed. Invalid index")
						break
					}
				} else {
					if idx == 0 {
						err = errors.New("Text transform failed. Invalid index")
						break
					}
				}

				// Adjust index
				idx--
			}

			pe := &this.dictList[idx]
			length := int(pe.data>>24) & 0xFF

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

			// Sanity check
			if pe.ptr == nil || dstIdx+length >= dstEnd {
				err = errors.New("Text transform failed. Invalid input data")
				break
			}

			// Emit word
			copy(dst[dstIdx:], pe.ptr[0:length])

			// Flip case of first character
			dst[dstIdx] ^= flipMask
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

					if dstIdx >= dstEnd {
						err = errors.New("Text transform failed. Invalid input data")
						break
					}
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

func (this *textCodec2) MaxEncodedLen(srcLen int) int {
	// Limit to 1 x srcLength and let the caller deal with
	// a failure when the output is too small
	return srcLen
}
