package db

type TenantSettings struct {
	WelcomeMessage           string `json:"welcomeMessage,omitempty"`
	BotName                  string `json:"botName,omitempty"`
	CancellationMsg          string `json:"cancellationMessage,omitempty"`
	ContactEmail             string `json:"contactEmail,omitempty"`
	OwnerPhone               string `json:"ownerPhone,omitempty"`
	LateCancelHours          int    `json:"lateCancelHours"`          // default: 2
	AutoBlockAfterNoShows    int    `json:"autoBlockAfterNoShows"`    // default: 3
	AutoBlockAfterLateCancel int    `json:"autoBlockAfterLateCancel"` // default: 3
	SendWarningBeforeBlock   bool   `json:"sendWarningBeforeBlock"`
}

type PlanFeatures struct {
	MaxServices             *int `json:"maxServices,omitempty"`
	MaxResources            *int `json:"maxResources,omitempty"`
	MaxAppointmentsPerMonth *int `json:"maxAppointmentsPerMonth,omitempty"`
	FeatureAnalytics        bool `json:"featureAnalytics"`
	FeatureBasicReminders   bool `json:"featureBasicReminders"`
	FeatureCustomReminders  bool `json:"featureCustomReminders"`
	FeatureMultiLocation    bool `json:"featureMultiLocation"`
	FeaturePrioritySupport  bool `json:"featurePrioritySupport"`
	FeaturePublicBooking    bool `json:"featurePublicBooking"`
	FeatureRecurring        bool `json:"featureRecurring"`
	FeatureWaitingList      bool `json:"featureWaitingList"`
}
