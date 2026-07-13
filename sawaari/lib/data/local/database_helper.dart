import 'dart:io';

import 'package:drift/drift.dart';
import 'package:drift/native.dart';
import 'package:path_provider/path_provider.dart';
import 'package:path/path.dart' as p;

part 'database_helper.g.dart';

/// Trip status enum
enum TripStatus {
  pending,
  inProgress,
  completed,
  cancelled,
}

/// User preferences model
class UserPreferences {
  final bool saheliMode;
  final bool acPreference;
  final bool nightMode;
  final String preferredPayment;

  const UserPreferences({
    this.saheliMode = false,
    this.acPreference = false,
    this.nightMode = false,
    this.preferredPayment = 'upi',
  });

  Map<String, dynamic> toJson() => {
        'saheliMode': saheliMode,
        'acPreference': acPreference,
        'nightMode': nightMode,
        'preferredPayment': preferredPayment,
      };

  factory UserPreferences.fromJson(Map<String, dynamic> json) {
    return UserPreferences(
      saheliMode: json['saheliMode'] as bool? ?? false,
      acPreference: json['acPreference'] as bool? ?? false,
      nightMode: json['nightMode'] as bool? ?? false,
      preferredPayment: json['preferredPayment'] as String? ?? 'upi',
    );
  }
}

/// Saved places table
class SavedPlaces extends Table {
  IntColumn get id => integer().autoIncrement()();
  TextColumn get name => text().withLength(min: 1, max: 100)();
  TextColumn get address => text()();
  RealColumn get latitude => real()();
  RealColumn get longitude => real()();
  TextColumn get placeType => text().withDefault(const Constant('other'))();
  BoolColumn get isFavorite => boolean().withDefault(const Constant(false))();
  DateTimeColumn get createdAt => dateTime().withDefault(currentDateAndTime)();
}

/// Trip history table
class TripHistory extends Table {
  IntColumn get id => integer().autoIncrement()();
  TextColumn get fromName => text()();
  RealColumn get fromLatitude => real()();
  RealColumn get fromLongitude => real()();
  TextColumn get toName => text()();
  RealColumn get toLatitude => real()();
  RealColumn get toLongitude => real()();
  TextColumn get mode => text()();
  TextColumn get provider => text().nullable()();
  IntColumn get fare => integer().nullable()();
  IntColumn get status => intEnum<TripStatus>()();
  DateTimeColumn get tripDate => dateTime()();
  DateTimeColumn get createdAt => dateTime().withDefault(currentDateAndTime)();
}

/// Bookings table
class Bookings extends Table {
  IntColumn get id => integer().autoIncrement()();
  TextColumn get bookingId => text().unique()();
  IntColumn get tripId => integer().nullable().references(TripHistory, #id)();
  TextColumn get mode => text()();
  TextColumn get provider => text().nullable()();
  IntColumn get fare => integer()();
  TextColumn get status => text().withDefault(const Constant('pending'))();
  TextColumn get ticketId => text().nullable()();
  TextColumn get qrPayload => text().nullable()();
  TextColumn get rail => text().withDefault(const Constant('deeplink'))();
  DateTimeColumn get bookingDate => dateTime()();
  DateTimeColumn get travelDate => dateTime().nullable()();
  DateTimeColumn get createdAt => dateTime().withDefault(currentDateAndTime)();
}

/// Fare cache table for offline fare estimation
class FareCache extends Table {
  IntColumn get id => integer().autoIncrement()();
  TextColumn get fromPlace => text()();
  TextColumn get toPlace => text()();
  TextColumn get mode => text()();
  IntColumn get estimatedFare => integer()();
  DateTimeColumn get validUntil => dateTime()();
  DateTimeColumn get cachedAt => dateTime().withDefault(currentDateAndTime)();
}

/// GTFS stops cache for offline nearby stops
class CachedStops extends Table {
  IntColumn get id => integer().autoIncrement()();
  TextColumn get stopId => text().unique()();
  TextColumn get stopName => text()();
  RealColumn get latitude => real()();
  RealColumn get longitude => real()();
  TextColumn get stopType => text().withDefault(const Constant('bus'))();
  DateTimeColumn get cachedAt => dateTime().withDefault(currentDateAndTime)();
}

/// User profile table
class UserProfiles extends Table {
  IntColumn get id => integer().autoIncrement()();
  TextColumn get phone => text().unique()();
  TextColumn get name => text().nullable()();
  TextColumn get preferences => text().withDefault(const Constant('{}'))();
  BoolColumn get saheliEnabled => boolean().withDefault(const Constant(false))();
  DateTimeColumn get createdAt => dateTime().withDefault(currentDateAndTime)();
  DateTimeColumn get updatedAt => dateTime().withDefault(currentDateAndTime)();
}

@DriftDatabase(tables: [SavedPlaces, TripHistory, Bookings, FareCache, CachedStops, UserProfiles])
class AppDatabase extends _$AppDatabase {
  AppDatabase() : super(_openConnection());

  @override
  int get schemaVersion => 1;

  @override
  MigrationStrategy get migration {
    return MigrationStrategy(
      onCreate: (Migrator m) async {
        await m.createAll();
      },
      onUpgrade: (Migrator m, int from, int to) async {
        // Handle migrations here
      },
      beforeOpen: (details) async {
        // Enable foreign keys
        await customStatement('PRAGMA foreign_keys = ON');
      },
    );
  }

  // Saved Places Operations
  Future<List<SavedPlace>> getAllSavedPlaces() => select(savedPlaces).get();

  Future<List<SavedPlace>> getFavoritePlaces() {
    return (select(savedPlaces)..where((t) => t.isFavorite.equals(true))).get();
  }

  Future<int> insertSavedPlace(SavedPlacesCompanion place) {
    return into(savedPlaces).insert(place);
  }

  Future<bool> updateSavedPlace(SavedPlace place) {
    return update(savedPlaces).replace(place);
  }

  Future<int> deleteSavedPlace(int id) {
    return (delete(savedPlaces)..where((t) => t.id.equals(id))).go();
  }

  // Trip History Operations
  Future<List<TripHistoryData>> getRecentTrips({int limit = 10}) {
    return (select(tripHistory)
          ..orderBy([(t) => OrderingTerm.desc(t.tripDate)])
          ..limit(limit))
        .get();
  }

  Future<int> insertTrip(TripHistoryCompanion trip) {
    return into(tripHistory).insert(trip);
  }

  Future<bool> updateTripStatus(int id, TripStatus status) {
    return (update(tripHistory)..where((t) => t.id.equals(id)))
        .write(TripHistoryCompanion(status: Value(status)));
  }

  // Booking Operations
  Future<List<Booking>> getAllBookings() {
    return (select(bookings)..orderBy([(t) => OrderingTerm.desc(t.bookingDate)])).get();
  }

  Future<List<Booking>> getUpcomingBookings() {
    final now = DateTime.now();
    return (select(bookings)
          ..where((t) => t.travelDate.isBiggerOrEqualValue(now))
          ..orderBy([(t) => OrderingTerm.asc(t.travelDate)]))
        .get();
  }

  Future<Booking?> getBookingById(String bookingId) {
    return (select(bookings)..where((t) => t.bookingId.equals(bookingId))).getSingleOrNull();
  }

  Future<int> insertBooking(BookingsCompanion booking) {
    return into(bookings).insert(booking);
  }

  Future<bool> updateBookingStatus(String bookingId, String status) {
    return (update(bookings)..where((t) => t.bookingId.equals(bookingId)))
        .write(BookingsCompanion(status: Value(status)));
  }

  // Fare Cache Operations
  Future<CachedFare?> getCachedFare(String from, String to, String mode) {
    return (select(fareCache)
          ..where((t) =>
              t.fromPlace.equals(from) &
              t.toPlace.equals(to) &
              t.mode.equals(mode) &
              t.validUntil.isBiggerOrEqualValue(DateTime.now())))
        .getSingleOrNull();
  }

  Future<void> cacheFare(FareCacheCompanion fare) async {
    // Delete old cache for same route
    await (delete(fareCache)
          ..where((t) =>
              t.fromPlace.equals(fare.fromPlace.value) &
              t.toPlace.equals(fare.toPlace.value) &
              t.mode.equals(fare.mode.value)))
        .go();
    await into(fareCache).insert(fare);
  }

  // Stops Cache Operations
  Future<List<CachedStop>> getNearbyCachedStops(double lat, double lng, double radiusKm) {
    // Simple bounding box query (not true distance calculation)
    final latDiff = radiusKm / 111.0; // ~111km per degree latitude
    final lngDiff = radiusKm / (111.0 * 0.75); // Rough approximation for Delhi latitude

    return (select(cachedStops)
          ..where((t) =>
              t.latitude.isBetweenValues(lat - latDiff, lat + latDiff) &
              t.longitude.isBetweenValues(lng - lngDiff, lng + lngDiff)))
        .get();
  }

  Future<void> cacheStops(List<CachedStopsCompanion> stops) async {
    await batch((batch) {
      batch.insertAll(cachedStops, stops, mode: InsertMode.insertOrReplace);
    });
  }

  // User Profile Operations
  Future<UserProfile?> getUserProfile(String phone) {
    return (select(userProfiles)..where((t) => t.phone.equals(phone))).getSingleOrNull();
  }

  Future<int> insertUserProfile(UserProfilesCompanion profile) {
    return into(userProfiles).insert(profile, mode: InsertMode.insertOrReplace);
  }

  Future<bool> updateUserPreferences(String phone, String preferencesJson) {
    return (update(userProfiles)..where((t) => t.phone.equals(phone)))
        .write(UserProfilesCompanion(
          preferences: Value(preferencesJson),
          updatedAt: Value(DateTime.now()),
        ));
  }

  // Clear all cached data
  Future<void> clearExpiredCache() async {
    await (delete(fareCache)..where((t) => t.validUntil.isSmallerThanValue(DateTime.now()))).go();
  }
}

LazyDatabase _openConnection() {
  return LazyDatabase(() async {
    final dbFolder = await getApplicationDocumentsDirectory();
    final file = File(p.join(dbFolder.path, 'sawaari.sqlite'));
    return NativeDatabase.createInBackground(file);
  });
}
