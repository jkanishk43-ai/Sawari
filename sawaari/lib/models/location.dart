import 'package:latlong2/latlong.dart';

class LocationData {
  final double latitude;
  final double longitude;
  final double? accuracy;
  final DateTime timestamp;
  final LocationSource source;

  const LocationData({
    required this.latitude,
    required this.longitude,
    this.accuracy,
    required this.timestamp,
    required this.source,
  });

  LatLng get latLng => LatLng(latitude, longitude);

  factory LocationData.fromJson(Map<String, dynamic> json) {
    return LocationData(
      latitude: (json['latitude'] as num).toDouble(),
      longitude: (json['longitude'] as num).toDouble(),
      accuracy: (json['accuracy'] as num?)?.toDouble(),
      timestamp: DateTime.parse(json['timestamp'] as String),
      source: LocationSource.values.firstWhere(
        (e) => e.name == json['source'],
        orElse: () => LocationSource.gps,
      ),
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'latitude': latitude,
      'longitude': longitude,
      'accuracy': accuracy,
      'timestamp': timestamp.toIso8601String(),
      'source': source.name,
    };
  }

  LocationData copyWith({
    double? latitude,
    double? longitude,
    double? accuracy,
    DateTime? timestamp,
    LocationSource? source,
  }) {
    return LocationData(
      latitude: latitude ?? this.latitude,
      longitude: longitude ?? this.longitude,
      accuracy: accuracy ?? this.accuracy,
      timestamp: timestamp ?? this.timestamp,
      source: source ?? this.source,
    );
  }
}

enum LocationSource {
  gps,
  navic,
  network,
  fused,
  cached,
}
