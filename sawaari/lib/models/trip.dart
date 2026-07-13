import 'location.dart';

class Trip {
  final String id;
  final LocationData from;
  final LocationData to;
  final String? fromAddress;
  final String? toAddress;
  final DateTime createdAt;
  final TripPreferences preferences;

  const Trip({
    required this.id,
    required this.from,
    required this.to,
    this.fromAddress,
    this.toAddress,
    required this.createdAt,
    required this.preferences,
  });

  factory Trip.fromJson(Map<String, dynamic> json) {
    return Trip(
      id: json['id'] as String,
      from: LocationData.fromJson(json['from'] as Map<String, dynamic>),
      to: LocationData.fromJson(json['to'] as Map<String, dynamic>),
      fromAddress: json['from_address'] as String?,
      toAddress: json['to_address'] as String?,
      createdAt: DateTime.parse(json['created_at'] as String),
      preferences: TripPreferences.fromJson(
          json['preferences'] as Map<String, dynamic>),
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'from': from.toJson(),
      'to': to.toJson(),
      'from_address': fromAddress,
      'to_address': toAddress,
      'created_at': createdAt.toIso8601String(),
      'preferences': preferences.toJson(),
    };
  }
}

class TripPreferences {
  final bool preferAc;
  final bool saheliDiscount;
  final bool nightRide;
  final int? maxStops;

  const TripPreferences({
    this.preferAc = false,
    this.saheliDiscount = false,
    this.nightRide = false,
    this.maxStops,
  });

  factory TripPreferences.fromJson(Map<String, dynamic> json) {
    return TripPreferences(
      preferAc: json['prefer_ac'] as bool? ?? false,
      saheliDiscount: json['saheli_discount'] as bool? ?? false,
      nightRide: json['night_ride'] as bool? ?? false,
      maxStops: json['max_stops'] as int?,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'prefer_ac': preferAc,
      'saheli_discount': saheliDiscount,
      'night_ride': nightRide,
      'max_stops': maxStops,
    };
  }

  TripPreferences copyWith({
    bool? preferAc,
    bool? saheliDiscount,
    bool? nightRide,
    int? maxStops,
  }) {
    return TripPreferences(
      preferAc: preferAc ?? this.preferAc,
      saheliDiscount: saheliDiscount ?? this.saheliDiscount,
      nightRide: nightRide ?? this.nightRide,
      maxStops: maxStops ?? this.maxStops,
    );
  }
}
