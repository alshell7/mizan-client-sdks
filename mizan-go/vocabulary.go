package mizan

// Capability is an entitlement key stored in an immutable plan snapshot.
type Capability string

const (
	CapabilityWhatsAppNumber1         Capability = "whatsapp_number_1"
	CapabilityInstagramAccount1       Capability = "instagram_account_1"
	CapabilityWebsiteWidget           Capability = "website_widget"
	CapabilityVoIP                    Capability = "voip"
	CapabilityIVR                     Capability = "ivr"
	CapabilityCallRecording30D        Capability = "call_recording_30d"
	CapabilityCallMonitoring          Capability = "call_monitoring"
	CapabilityAICallSummary           Capability = "ai_call_summary"
	CapabilityUnifiedInbox            Capability = "unified_inbox"
	CapabilityMobileApp               Capability = "mobile_app"
	CapabilityAIAgentBuilder          Capability = "ai_agent_builder"
	CapabilityBasicAnalytics          Capability = "basic_analytics"
	CapabilityBasicBroadcasts         Capability = "basic_broadcasts"
	CapabilityBasicContacts           Capability = "basic_contacts"
	CapabilityQuickReplies            Capability = "quick_replies"
	CapabilityWelcomeWorkingHours     Capability = "welcome_working_hours"
	CapabilityMentions                Capability = "mentions"
	CapabilityPrivateNotes            Capability = "private_notes"
	CapabilityEcommerceStore1         Capability = "ecommerce_store_1"
	CapabilityEcommerceApp            Capability = "ecommerce_app"
	CapabilityWebhooksAPI             Capability = "webhooks_api"
	CapabilityNotificationLog         Capability = "notification_log"
	CapabilityAzeerSupport            Capability = "azeer_support"
	CapabilitySecurityBaseline        Capability = "security_baseline"
	CapabilityTwoFactorAuthentication Capability = "two_factor_authentication"
	CapabilityBYOLSIP                 Capability = "byol_sip"
	CapabilityCustomAttributes        Capability = "custom_attributes"
	CapabilityAdvancedContacts        Capability = "advanced_contacts"
	CapabilityCustomInbox             Capability = "custom_inbox"
	CapabilityMediaLibrary            Capability = "media_library"
	CapabilityClosingReasons          Capability = "closing_reasons"
	CapabilityBulkHistory             Capability = "bulk_history"
	CapabilityCSAT                    Capability = "csat"
	CapabilityAdvancedAnalytics       Capability = "advanced_analytics"
	CapabilityWhatsAppFlow            Capability = "whatsapp_flow"
	CapabilitySegmentedCampaigns      Capability = "segmented_campaigns"
	CapabilitySegmentedRouting        Capability = "segmented_routing"
	CapabilityWhatsAppCatalog         Capability = "whatsapp_catalog"
	CapabilityMultiStore              Capability = "multi_store"
	CapabilityAdvancedEcommerce       Capability = "advanced_ecommerce"
	CapabilityMasking                 Capability = "masking"
	CapabilityAdvancedAIIntegrations  Capability = "advanced_ai_integrations"
	CapabilityWhatsAppFlowAPI         Capability = "whatsapp_flow_api"
	CapabilityCustomBroadcastLogic    Capability = "custom_broadcast_logic"
	CapabilityExternalAttributeSync   Capability = "external_attribute_sync"
	CapabilityAdvancedAttributeTypes  Capability = "advanced_attribute_types"
	CapabilityCustomAutomationRules   Capability = "custom_automation_rules"
	CapabilityRBACAudit               Capability = "rbac_audit"
	CapabilityWhiteLabel              Capability = "white_label"
	CapabilityLocalHosting            Capability = "local_hosting"
)

func AllPlanIDs() []PlanID { return []PlanID{PlanStart, PlanGrowth, PlanCommand} }
func AllBillingTerms() []BillingTerm {
	return []BillingTerm{TermMonthly, TermQuarterly, TermSemiAnnual, TermAnnual}
}
func AllFeatureCodes() []FeatureCode {
	return []FeatureCode{FeatureConversation24H, FeatureOutboundDeliveredMessage, FeatureAIAssistOverAllowance, FeatureVoiceAIStartedMinute, FeatureAIReplyHandling, FeatureWhatsAppMetaMarketingMessage, FeatureTelephonyVoiceMinute, FeatureInboundVoiceMinute, FeatureOtherProviderCharge}
}
func AllChannels() []Channel {
	return []Channel{ChannelWhatsApp, ChannelInstagram, ChannelFacebook, ChannelTikTok, ChannelTelephony, ChannelWebchat}
}
