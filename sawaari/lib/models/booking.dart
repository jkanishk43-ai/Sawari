import 'ride_option.dart';

enum BookingStatus {
  pending,
  confirmed,
  inProgress,
  completed,
  cancelled,
  failed,
}

enum PaymentMethod {
  upi,
  wallet,
  card,
  cash,
  saheliCard,
}

class Booking {
  final String id;
  final String tripId;
  final RideOption rideOption;
  final String pickupAddress;
  final String dropoffAddress;
  final double pickupLatitude;
  final double pickupLongitude;
  final double dropoffLatitude;
  final double dropoffLongitude;
  final DateTime createdAt;
  final DateTime? scheduledAt;
  final BookingStatus status;
  final PaymentMethod paymentMethod;
  final double finalFare;
  final String? ticketId;
  final String? qrPayload;
  final String? ondcTransactionId;

  const Booking({
    required this.id,
    required this.tripId,
    required this.rideOption,
    required this.pickupAddress,
    required this.dropoffAddress,
    required this.pickupLatitude,
    required this.pickupLongitude,
    required this.dropoffLatitude,
    required this.dropoffLongitude,
    required this.createdAt,
    this.scheduledAt,
    required this.status,
    required this.paymentMethod,
    required this.finalFare,
    this.ticketId,
    this.qrPayload,
    this.ondcTransactionId,
  });

  factory Booking.fromJson(Map<String, dynamic> json) {
    return Booking(
      id: json['id'] as String,
      tripId: json['trip_id'] as String,
      rideOption: RideOption.fromJson(json['ride_option'] as Map<String, dynamic>),
      pickupAddress: json['pickup_address'] as String,
      dropoffAddress: json['dropoff_address'] as String,
      pickupLatitude: (json['pickup_latitude'] as num).toDouble(),
      pickupLongitude: (json['pickup_longitude'] as num).toDouble(),
      dropoffLatitude: (json['dropoff_latitude'] as num).toDouble(),
      dropoffLongitude: (json['dropoff_longitude'] as num).toDouble(),
      createdAt: DateTime.parse(json['created_at'] as String),
      scheduledAt: json['scheduled_at'] != null
          ? DateTime.parse(json['scheduled_at'] as String)
          : null,
      status: BookingStatus.values.firstWhere(
        (e) => e.name == json['status'],
        orElse: () => BookingStatus.pending,
      ),
      paymentMethod: PaymentMethod.values.firstWhere(
        (e) => e.name == json['payment_method'],
        orElse: () => PaymentMethod.upi,
      ),
      finalFare: (json['final_fare'] as num).toDouble(),
      ticketId: json['ticket_id'] as String?,
      qrPayload: json['qr_payload'] as String?,
      ondcTransactionId: json['ondc_transaction_id'] as String?,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'trip_id': tripId,
      'ride_option': rideOption.toJson(),
      'pickup_address': pickupAddress,
      'dropoff_address': dropoffAddress,
      'pickup_latitude': pickupLatitude,
      'pickup_longitude': pickupLongitude,
      'dropoff_latitude': dropoffLatitude,
      'dropoff_longitude': dropoffLongitude,
      'created_at': createdAt.toIso8601String(),
      'scheduled_at': scheduledAt?.toIso8601String(),
      'status': status.name,
      'payment_method': paymentMethod.name,
      'final_fare': finalFare,
      'ticket_id': ticketId,
      'qr_payload': qrPayload,
      'ondc_transaction_id': ondcTransactionId,
    };
  }
}
