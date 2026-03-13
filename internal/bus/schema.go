package bus

const (
	TopicPaymentStatusUpdate Topic = "payment.status"
	TopicFormSubmissions     Topic = "form.submitted"
	TopicUserRegistered      Topic = "user.registered"
)

type PaymentStatusUpdateEvent struct {
	MerchantID int64
	PaymentID  int64
}

type FormSubmittedEvent struct {
	RequestType string
	Message     string
	UserID      int64
}

type UserRegisteredEvent struct {
	UserID int64
}
