param containerName string = 'adventureguild'

resource cosmosdb 'Microsoft.DocumentDb/databaseAccounts@2020-04-01' = {
  kind: 'MongoDB'
  name: containerName
  location: 'westeurope'
  properties: {
    databaseAccountOfferType: 'Standard'
    locations: [
      {
        id: '${containerName}-westeurope'
        failoverPriority: 0
        locationName: 'westeurope'
      }
    ]
    backupPolicy: {
      type: 'Periodic'
      periodicModeProperties: {
        backupIntervalInMinutes: 1440
        backupRetentionIntervalInHours: 48
      }
    }
    isVirtualNetworkFilterEnabled: false
    virtualNetworkRules: []
    ipRules: []
    dependsOn: []
    enableMultipleWriteLocations: false
    capabilities: [
      {
        name: 'EnableMongo'
      }
      {
        name: 'DisableRateLimitingResponses'
      }
    ]
    apiProperties: {
      serverVersion: '4.0'
    }
    enableFreeTier: false
  }
  tags: {
    defaultExperience: 'Azure Cosmos DB for MongoDB API'
  }
}
