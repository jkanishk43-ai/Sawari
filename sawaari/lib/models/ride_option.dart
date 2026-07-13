enum RideCategory {
  bus,
  metro,
  auto,
  cab,
  bike,
}

class RideOption {
  final String id;
  final String provider;
  final String name;
  final RideCategory category;
  final String description;
  final double fareMin;
  final double fareMax;
  final Duration eta;
  final Duration travelTime;
  final double reliabilityScore;
  final RideBadge? badge;
  final FareBreakdown? fareBreakdown;
  final String? bookingRail;
  final String? deeplinkUrl;

  const RideOption({
    required this.id,
    required this.provider,
    required this.name,
    required this.category,
    required this.description,
    required this.fareMin,
    required this.fareMax,
    required this.eta,
    required this.travelTime,
    required this.reliabilityScore,
    this.badge,
    this.fareBreakdown,
    this.bookingRail,
    this.deeplinkUrl,
  });

  String get formattedFare {
    if (fareMin == fareMax) {
      return '₹${fareMin.toInt()}';
    }
    return '₹${fareMin.toInt()} - ₹${fareMax.toInt()}';
  }

  String get formattedEta {
    if (eta.inMinutes < 1) {
      return 'Arriving';
    }
    return '${eta.inMinutes} min';
  }

  String get formattedTravelTime {
    if (travelTime.inHours > 0) {
      final hours = travelTime.inHours;
      final mins = travelTime.inMinutes % 60;
      return '${hours}h ${mins}m';
    }
    return '${travelTime.inMinutes} min';
  }

  double get score {
    // SMART PICK algorithm: 0.55 * fare + 0.45 * eta
    const maxFare = 500.0;
    const maxEta = 60.0;
    final fareScore = fareMin / maxFare;
    final etaScore = eta.inMinutes / maxEta;
    return 0.55 * fareScore + 0.45 * etaScore;
  }

  factory RideOption.fromJson(Map<String, dynamic> json) {
    return RideOption(
      id: json['id'] as String,
      provider: json['provider'] as String,
      name: json['name'] as String,
      category: RideCategory.values.firstWhere(
        (e) => e.name == json['category'],
        orElse: () => RideCategory.cab,
      ),
      description: json['description'] as String,
      fareMin: (json['fare_min'] as num).toDouble(),
      fareMax: (json['fare_max'] as num).toDouble(),
      eta: Duration(seconds: json['eta_seconds'] as int),
      travelTime: Duration(seconds: json['travel_time_seconds'] as int),
      reliabilityScore: (json['reliability_score'] as num).toDouble(),
      badge: json['badge'] != null
          ? RideBadge.values.firstWhere(
              (e) => e.name == json['badge'],
              orElse: () => RideBadge.cheapest,
            )
          : null,
      fareBreakdown: json['fare_breakdown'] != null
          ? FareBreakdown.fromJson(json['fare_breakdown'] as Map<String, dynamic>)
          : null,
      bookingRail: json['booking_rail'] as String?,
      deeplinkUrl: json['deeplink_url'] as String?,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'provider': provider,
      'name': name,
      'category': category.name,
      'description': description,
      'fare_min': fareMin,
      'fare_max': fareMax,
      'eta_seconds': eta.inSeconds,
      'travel_time_seconds': travelTime.inSeconds,
      'reliability_score': reliabilityScore,
      'badge': badge?.name,
      'fare_breakdown': fareBreakdown?.toJson(),
      'booking_rail': bookingRail,
      'deeplink_url': deeplinkUrl,
    };
  }
}

enum RideBadge {
  cheapest,
  fastest,
  smartPick,
  comfort,
  saheli,
}

class FareBreakdown {
  final double baseFare;
  final double distanceCharge;
  final double timeCharge;
  final double surge;
  final double discount;
  final double saheliDiscount;

  const FareBreakdown({
    required this.baseFare,
    required this.distanceCharge,
    required this.timeCharge,
    this.surge = 0,
    this.discount = 0,
    this.saheliDiscount = 0,
  });

  double get total =>
      baseFare + distanceCharge + timeCharge + surge - discount - saheliDiscount;

  factory FareBreakdown.fromJson(Map<String, dynamic> json) {
    return FareBreakdown(
      baseFare: (json['base_fare'] as num).toDouble(),
      distanceCharge: (json['distance_charge'] as num).toDouble(),
      timeCharge: (json['time_charge'] as num).toDouble(),
      surge: (json['surge'] as num?)?.toDouble() ?? 0,
      discount: (json['discount'] as num?)?.toDouble() ?? 0,
      saheliDiscount: (json['saheli_discount'] as num?)?.toDouble() ?? 0,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'base_fare': baseFare,
      'distance_charge': distanceCharge,
      'time_charge': timeCharge,
      'surge': surge,
      'discount': discount,
      'saheli_discount': saheliDiscount,
    };
  }
}
