package custom_request

type GetCouponResponse struct {
	Error     string `json:"Error,omitempty"`
	ErrorCode int    `json:"ErrorCode,omitempty"`
	GUID      string `json:"Guid,omitempty"`
	ID        int    `json:"Id,omitempty"`
	Success   bool   `json:"Success,omitempty"`
	Value     *struct {
		AntiExpressCoef int `json:"AntiExpressCoef,omitempty"`
		BonusCode       any `json:"BonusCode,omitempty"`
		CfView          int `json:"CfView,omitempty"`
		CheckCf         int `json:"CheckCf,omitempty"`
		Code            int `json:"Code,omitempty"`
		Coef            int `json:"Coef,omitempty"`
		Country         any `json:"Country,omitempty"`
		Currency        int `json:"Currency,omitempty"`
		CustomerID      any `json:"CustomerId,omitempty"`
		DebitFrom       any `json:"DebitFrom,omitempty"`
		Events          []struct {
			Type            string  `json:"__type,omitempty"`
			Block           bool    `json:"Block,omitempty"`
			Cv              any     `json:"CV,omitempty"`
			ChampID         int     `json:"ChampId,omitempty"`
			Coef            float64 `json:"Coef,omitempty"`
			Expired         int     `json:"Expired,omitempty"`
			ExtraKind       int     `json:"ExtraKind,omitempty"`
			Fs1             any     `json:"FS1,omitempty"`
			Fs2             any     `json:"FS2,omitempty"`
			Finish          bool    `json:"Finish,omitempty"`
			FullName        any     `json:"FullName,omitempty"`
			FullScore       int     `json:"FullScore,omitempty"`
			GameConstID     int     `json:"GameConstId,omitempty"`
			GameID          int     `json:"GameId,omitempty"`
			GroupName       string  `json:"GroupName,omitempty"`
			InstrumentID    int     `json:"InstrumentId,omitempty"`
			IsBannedExpress bool    `json:"IsBannedExpress,omitempty"`
			IsRelation      int     `json:"IsRelation,omitempty"`
			Kind            int     `json:"Kind,omitempty"`
			Liga            string  `json:"Liga,omitempty"`
			Ms1             any     `json:"MS1,omitempty"`
			Ms2             any     `json:"MS2,omitempty"`
			MainGameID      int     `json:"MainGameId,omitempty"`
			MarketName      string  `json:"MarketName,omitempty"`
			Opp1            string  `json:"Opp1,omitempty"`
			Opp1ID          int     `json:"Opp1Id,omitempty"`
			Opp2            string  `json:"Opp2,omitempty"`
			Opp2ID          int     `json:"Opp2Id,omitempty"`
			Ps              any     `json:"PS,omitempty"`
			Pv              any     `json:"PV,omitempty"`
			Param           int     `json:"Param,omitempty"`
			PeriodName      string  `json:"PeriodName,omitempty"`
			PeriodScores    any     `json:"PeriodScores,omitempty"`
			PlayerID        int     `json:"PlayerId,omitempty"`
			Price           int     `json:"Price,omitempty"`
			Seconds         any     `json:"Seconds,omitempty"`
			SpecialCoef     int     `json:"SpecialCoef,omitempty"`
			SportID         int     `json:"SportId,omitempty"`
			Start           int     `json:"Start,omitempty"`
			TimeDirection   int     `json:"TimeDirection,omitempty"`
			TimeSec         int     `json:"TimeSec,omitempty"`
			Type0           int     `json:"Type,omitempty"`
			DateStart       string  `json:"DateStart,omitempty"`
			GameType        any     `json:"GameType,omitempty"`
			GameVid         any     `json:"GameVid,omitempty"`
			GroupID         int     `json:"GroupId,omitempty"`
			Number          int     `json:"Number,omitempty"`
			PlayerName      string  `json:"PlayerName,omitempty"`
			ShortGroupID    int     `json:"ShortGroupId,omitempty"`
			SportName       string  `json:"SportName,omitempty"`
		} `json:"Events,omitempty"`
		EventsIndexes   any    `json:"EventsIndexes,omitempty"`
		ExpresCoef      int    `json:"ExpresCoef,omitempty"`
		Groups          any    `json:"Groups,omitempty"`
		GroupsSumms     any    `json:"GroupsSumms,omitempty"`
		IsMatchOfDay    bool   `json:"IsMatchOfDay,omitempty"`
		IsPowerBet      bool   `json:"IsPowerBet,omitempty"`
		Kind            int    `json:"Kind,omitempty"`
		Lng             string `json:"Lng,omitempty"`
		ManagerID       any    `json:"ManagerId,omitempty"`
		MinBetSystem    string `json:"MinBetSystem,omitempty"`
		NeedUpdateLine  bool   `json:"NeedUpdateLine,omitempty"`
		ResultCoef      any    `json:"ResultCoef,omitempty"`
		ResultCoefView  any    `json:"ResultCoefView,omitempty"`
		SaleBetID       int    `json:"SaleBetId,omitempty"`
		Source          int    `json:"Source,omitempty"`
		Sport           int    `json:"Sport,omitempty"`
		Summ            int    `json:"Summ,omitempty"`
		TerminalCode    any    `json:"TerminalCode,omitempty"`
		TerminalCodeWeb any    `json:"TerminalCodeWeb,omitempty"`
		Top             int    `json:"Top,omitempty"`
		UserID          int    `json:"UserId,omitempty"`
		UserIDBonus     int    `json:"UserIdBonus,omitempty"`
		Vid             int    `json:"Vid,omitempty"`
		WithLobby       bool   `json:"WithLobby,omitempty"`
		AvanceBet       bool   `json:"avanceBet,omitempty"`
		BetGUID         any    `json:"betGUID,omitempty"`
		ChangeCf        bool   `json:"changeCf,omitempty"`
		ExceptionText   any    `json:"exceptionText,omitempty"`
		ExpressNum      int    `json:"expressNum,omitempty"`
		Fcountry        int    `json:"fcountry,omitempty"`
		MaxBet          int    `json:"maxBet,omitempty"`
		MinBet          int    `json:"minBet,omitempty"`
		NotLogin        bool   `json:"notLogin,omitempty"`
		NotWait         bool   `json:"notWait,omitempty"`
		Partner         int    `json:"partner,omitempty"`
		Promo           any    `json:"promo,omitempty"`
		PromoCodes      any    `json:"promoCodes,omitempty"`
		Description     any    `json:"Description,omitempty"`
		HasRemoveEvents bool   `json:"HasRemoveEvents,omitempty"`
	} `json:"Value,omitempty"`
	Source *Source `json:"source,omitempty"`
}

type Source struct {
	Type       string `json:"source_type,omitempty"`
	Source     string `json:"source_id,omitempty"`
	SourceName string `json:"source_name,omitempty"`
}
