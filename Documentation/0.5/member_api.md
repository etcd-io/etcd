## Member HTTP API

### Add Member

	PUT /v2/admin/members/<id>

		In Form:
			PeerURLs - advertised peer url of new member
		Code:
			201 Created - new member is added successfully
			400 BadRequest - id cannot be parsed out
			500 InternalServerError - cluster fails to commit proposal in timeout

*NOTE*: `id` is a hexadecimal number without leading 0x.

### Remove Member

	DELETE /v2/admin/members/<id>

		Code:
			204 NoContent - member is removed successfully
			400 BadRequest - id cannot be parsed out
			500 InternalServerError - cluster fails to commit proposal in timeout
