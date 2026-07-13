import { NextRequest, NextResponse } from 'next/server';

const BACKEND_URL = process.env.BACKEND_URL || 'http://localhost:8080';

export async function GET(request: NextRequest) {
  const searchParams = request.nextUrl.searchParams;
  const lat = searchParams.get('lat');
  const lng = searchParams.get('lng');
  const radius = searchParams.get('radius') || '1000';
  const type = searchParams.get('type'); // metro, bus, all

  try {
    const params = new URLSearchParams();
    if (lat) params.set('lat', lat);
    if (lng) params.set('lng', lng);
    params.set('radius', radius);
    if (type) params.set('type', type);

    const response = await fetch(
      `${BACKEND_URL}/api/stops?${params.toString()}`,
      {
        signal: AbortSignal.timeout,
      }
    );

    if (!response.ok) {
      throw new Error(`Backend responded with status ${response.status}`);
    }

    const data = await response.json();
    return NextResponse.json(data);
  } catch (error) {
    console.error('Stops API error:', error);

    // Return mock data for demo
    return NextResponse.json(getMockStopsResponse(), {
      headers: {
        'X-Data-Source': 'mock',
      },
    });
  }
}

function getMockStopsResponse() {
  return {
    success: true,
    dataSource: 'mock',
    stops: [
      {
        id: 'rajiv-chowk-metro',
        name: 'Rajiv Chowk Metro Station',
        type: 'metro',
        lines: ['Blue Line', 'Yellow Line'],
        distance: 300,
        location: { lat: 28.6328, lng: 77.2197 },
        amenities: ['WiFi', 'Elevator', 'AC'],
      },
      {
        id: 'barakhamba-bus',
        name: 'Barakhamba Road Bus Stop',
        type: 'bus',
        lines: ['DTC 114', 'Cluster 12'],
        distance: 500,
        location: { lat: 28.6299, lng: 77.2162 },
        amenities: ['Shelter', 'Benches'],
      },
      {
        id: 'mandi-house-metro',
        name: 'Mandi House Metro Station',
        type: 'metro',
        lines: ['Blue Line', 'Violet Line'],
        distance: 700,
        location: { lat: 28.6253, lng: 77.2075 },
        amenities: ['WiFi', 'Elevator'],
      },
      {
        id: 'ito-bus',
        name: 'ITO Bus Stop',
        type: 'bus',
        lines: ['DTC 44', 'Cluster 5'],
        distance: 900,
        location: { lat: 28.6178, lng: 77.2233 },
        amenities: ['Shelter'],
      },
    ],
  };
}
