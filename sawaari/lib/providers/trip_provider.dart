import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../models/models.dart';
import '../data/api/api_client.dart';

/// Current trip state
class TripState {
  final LocationData? from;
  final LocationData? to;
  final String? fromAddress;
  final String? toAddress;
  final TripPreferences preferences;
  final bool isComparing;
  final String? error;

  const TripState({
    this.from,
    this.to,
    this.fromAddress,
    this.toAddress,
    this.preferences = const TripPreferences(),
    this.isComparing = false,
    this.error,
  });

  TripState copyWith({
    LocationData? from,
    LocationData? to,
    String? fromAddress,
    String? toAddress,
    TripPreferences? preferences,
    bool? isComparing,
    String? error,
  }) {
    return TripState(
      from: from ?? this.from,
      to: to ?? this.to,
      fromAddress: fromAddress ?? this.fromAddress,
      toAddress: toAddress ?? this.toAddress,
      preferences: preferences ?? this.preferences,
      isComparing: isComparing ?? this.isComparing,
      error: error,
    );
  }

  bool get isComplete => from != null && to != null;
}

/// Trip notifier for managing trip state
class TripNotifier extends StateNotifier<TripState> {
  final ApiClient _apiClient;

  TripNotifier(this._apiClient) : super(const TripState());

  void setFrom(LocationData location, {String? address}) {
    state = state.copyWith(from: location, fromAddress: address);
  }

  void setTo(LocationData location, {String? address}) {
    state = state.copyWith(to: location, toAddress: address);
  }

  void setPreferences(TripPreferences preferences) {
    state = state.copyWith(preferences: preferences);
  }

  void toggleSaheliDiscount() {
    state = state.copyWith(
      preferences: state.preferences.copyWith(
        saheliDiscount: !state.preferences.saheliDiscount,
      ),
    );
  }

  void toggleAcPreference() {
    state = state.copyWith(
      preferences: state.preferences.copyWith(
        preferAc: !state.preferences.preferAc,
      ),
    );
  }

  void clearTrip() {
    state = const TripState();
  }
}

/// API client provider
final apiClientProvider = Provider<ApiClient>((ref) {
  return ApiClient();
});

/// Trip provider
final tripProvider = StateNotifierProvider<TripNotifier, TripState>((ref) {
  final apiClient = ref.watch(apiClientProvider);
  return TripNotifier(apiClient);
});

/// Provider for comparing trip prices
final comparisonProvider = FutureProvider.family<List<RideOption>, TripState>((ref, trip) async {
  if (!trip.isComplete) {
    return [];
  }

  final apiClient = ref.read(apiClientProvider);

  try {
    return await apiClient.comparePrices(
      fromLat: trip.from!.latitude,
      fromLng: trip.from!.longitude,
      toLat: trip.to!.latitude,
      toLng: trip.to!.longitude,
      preferences: trip.preferences,
    );
  } catch (e) {
    // Return mock data for demo
    return _getMockRideOptions();
  }
});

/// Mock ride options for demo
List<RideOption> _getMockRideOptions() {
  return [
    const RideOption(
      id: 'bus_1',
      provider: 'DTC',
      name: 'Bus',
      category: RideCategory.bus,
      description: 'DTC / Cluster',
      fareMin: 15,
      fareMax: 25,
      eta: Duration(minutes: 10),
      travelTime: Duration(minutes: 45),
      reliabilityScore: 0.85,
      badge: RideBadge.cheapest,
    ),
    const RideOption(
      id: 'metro_1',
      provider: 'DMRC',
      name: 'Metro',
      category: RideCategory.metro,
      description: 'DMRC Blue Line',
      fareMin: 30,
      fareMax: 60,
      eta: Duration(minutes: 5),
      travelTime: Duration(minutes: 35),
      reliabilityScore: 0.95,
      badge: RideBadge.fastest,
    ),
    const RideOption(
      id: 'auto_1',
      provider: 'Meter',
      name: 'Auto',
      category: RideCategory.auto,
      description: 'Meter fare',
      fareMin: 80,
      fareMax: 150,
      eta: Duration(minutes: 3),
      travelTime: Duration(minutes: 30),
      reliabilityScore: 0.70,
    ),
    const RideOption(
      id: 'uber_go',
      provider: 'Uber',
      name: 'Uber Go',
      category: RideCategory.cab,
      description: 'Affordable, compact rides',
      fareMin: 120,
      fareMax: 200,
      eta: Duration(minutes: 4),
      travelTime: Duration(minutes: 25),
      reliabilityScore: 0.88,
    ),
    const RideOption(
      id: 'uber_premier',
      provider: 'Uber',
      name: 'Uber Premier',
      category: RideCategory.cab,
      description: 'Top rated drivers',
      fareMin: 200,
      fareMax: 350,
      eta: Duration(minutes: 6),
      travelTime: Duration(minutes: 25),
      reliabilityScore: 0.90,
      badge: RideBadge.comfort,
    ),
  ];
}
