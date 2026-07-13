import 'package:latlong2/latlong.dart';

class TransitStop {
  final String id;
  final String name;
  final double latitude;
  final double longitude;
  final StopType type;
  final List<String> routes;
  final double? distance;
  final DateTime? nextDeparture;

  const TransitStop({
    required this.id,
    required this.name,
    required this.latitude,
    required this.longitude,
    required this.type,
    this.routes = const [],
    this.distance,
    this.nextDeparture,
  });

  LatLng get latLng => LatLng(latitude, longitude);

  factory TransitStop.fromJson(Map<String, dynamic> json) {
    return TransitStop(
      id: json['id'] as String,
      name: json['name'] as String,
      latitude: (json['latitude'] as num).toDouble(),
      longitude: (json['longitude'] as num).toDouble(),
      type: StopType.values.firstWhere(
        (e) => e.name == json['type'],
        orElse: () => StopType.bus,
      ),
      routes: (json['routes'] as List<dynamic>?)?.cast<String>() ?? [],
      distance: (json['distance'] as num?)?.toDouble(),
      nextDeparture: json['next_departure'] != null
          ? DateTime.parse(json['next_departure'] as String)
          : null,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'name': name,
      'latitude': latitude,
      'longitude': longitude,
      'type': type.name,
      'routes': routes,
      'distance': distance,
      'next_departure': nextDeparture?.toIso8601String(),
    };
  }
}

enum StopType {
  bus,
  metro,
  auto,
  rickshaw,
}
